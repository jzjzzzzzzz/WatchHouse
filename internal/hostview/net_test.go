package hostview

import (
	"strings"
	"testing"
)

const header = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"

func TestParseListeningSockets(t *testing.T) {
	ipv4 := header +
		"   0: 0100007F:01BB 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000 0 4242 1 0000000000000000\n" +
		"   1: 00000000:0016 00000000:0000 01 00000000:00000000 00:00000000 00000000  0 0 9999\n"
	got, err := ParseProcNetTCP(strings.NewReader(ipv4), "ipv4")
	if err != nil || len(got) != 1 {
		t.Fatalf("parse: %+v %v", got, err)
	}
	if got[0].LocalAddress != "127.0.0.1" || got[0].LocalPort != 443 || got[0].KernelUID != 1000 || got[0].Inode != 4242 {
		t.Fatalf("unexpected socket: %+v", got[0])
	}

	ipv6 := header + "0: 00000000000000000000000001000000:20FB 00000000000000000000000000000000:0000 0A 0:0 00:0 0 0 0 77\n"
	got, err = ParseProcNetTCP(strings.NewReader(ipv6), "ipv6")
	if err != nil || len(got) != 1 || got[0].LocalAddress != "::1" || got[0].LocalPort != 8443 || got[0].Inode != 77 {
		t.Fatalf("IPv6 socket: %+v %v", got, err)
	}
}

func TestMalformedListenerFailsClosed(t *testing.T) {
	cases := []string{
		"",
		header + "short\n",
		header + "0: bad:0016 0:0 0A 0:0 00:0 0 0 0 4\n",
		header + "0: 0100007F:0016 0:0 0A 0:0 00:0 0 bad 0 4\n",
		header + "0: 0100007F:0016 0:0 0A 0:0 00:0 0 0 0 0\n",
		header + strings.Repeat("x", maxProcNetLine+1),
	}
	for i, input := range cases {
		if _, err := ParseProcNetTCP(strings.NewReader(input), "ipv4"); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	if _, err := ParseProcNetTCP(strings.NewReader(header), "other"); err == nil {
		t.Fatal("unsupported family accepted")
	}
}

func TestListenerBound(t *testing.T) {
	var input strings.Builder
	input.WriteString(header)
	for i := 0; i <= maxSockets; i++ {
		input.WriteString("0: 0100007F:0016 0:0 0A 0:0 00:0 0 0 0 4\n")
	}
	if _, err := ParseProcNetTCP(strings.NewReader(input.String()), "ipv4"); err == nil {
		t.Fatal("listener bound not enforced")
	}
}
