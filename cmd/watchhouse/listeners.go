package main

import (
	"encoding/json"
	"fmt"
	"io"

	"watchhouse/internal/hostview"
)

func runListeners(args []string, out, errOut io.Writer, snapshot func() (hostview.HostSnapshot, error)) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "listeners accepts no paths, PIDs, namespaces, or commands")
		return 2
	}
	result, err := snapshot()
	if err != nil {
		fmt.Fprintln(errOut, "listeners:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(errOut, "listeners: encode result:", err)
		return 1
	}
	return 0
}
