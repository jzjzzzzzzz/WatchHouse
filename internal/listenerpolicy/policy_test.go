package listenerpolicy

import (
	"fmt"
	"strings"
	"testing"
)

const validPolicy = `{"schema_version":1,"policy_id":"edge-v1","listeners":[{"resource_id":"https","family":"ipv4","local_address":"127.0.0.1","local_port":443,"allowed_units":["nginx.service"]}]}`

func TestDecodeStrictListenerPolicy(t *testing.T) {
	policy, err := Decode(strings.NewReader(validPolicy))
	if err != nil || policy.PolicyID != "edge-v1" || len(policy.Listeners) != 1 {
		t.Fatalf("policy %+v %v", policy, err)
	}
}

func TestInvalidPoliciesRejected(t *testing.T) {
	cases := []string{
		`{}`,
		validPolicy + `{}`,
		strings.Replace(validPolicy, `"policy_id":"edge-v1"`, `"policy_id":"edge-v1","policy_id":"other"`, 1),
		strings.Replace(validPolicy, `"policy_id":"edge-v1"`, `"unknown":true,"policy_id":"edge-v1"`, 1),
		strings.Replace(validPolicy, `"127.0.0.1"`, `"127.000.0.1"`, 1),
		strings.Replace(validPolicy, `"ipv4"`, `"ipv6"`, 1),
		strings.Replace(validPolicy, `443`, `0`, 1),
		strings.Replace(validPolicy, `"nginx.service"`, `"../nginx.service"`, 1),
		strings.Replace(validPolicy, `}]}`, `},{"resource_id":"https","family":"ipv4","local_address":"127.0.0.2","local_port":444,"allowed_units":[]}]}`, 1),
	}
	for index, body := range cases {
		if _, err := Decode(strings.NewReader(body)); err == nil {
			t.Fatalf("case %d accepted: %s", index, body)
		}
	}
}

func TestPolicyBounds(t *testing.T) {
	if _, err := Decode(strings.NewReader(strings.Repeat(" ", maxPolicyBytes+1))); err == nil {
		t.Fatal("byte bound not enforced")
	}
	declarations := make([]string, maxListeners+1)
	for i := range declarations {
		declarations[i] = fmt.Sprintf(`{"resource_id":"r%d","family":"ipv4","local_address":"127.0.0.1","local_port":%d,"allowed_units":[]}`, i, i+1)
	}
	body := fmt.Sprintf(`{"schema_version":1,"policy_id":"large","listeners":[%s]}`, strings.Join(declarations, ","))
	if _, err := Decode(strings.NewReader(body)); err == nil {
		t.Fatal("declaration bound not enforced")
	}
}
