package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"watchhouse/internal/hostview"
	"watchhouse/internal/listenerpolicy"
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

func runListenerCheck(args []string, out, errOut io.Writer, snapshot func() (hostview.HostSnapshot, error)) int {
	flags := flag.NewFlagSet("listener-check", flag.ContinueOnError)
	flags.SetOutput(errOut)
	policyPath := flags.String("policy", "", "strict listener policy JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *policyPath == "" {
		fmt.Fprintln(errOut, "listener-check requires --policy FILE and no positional arguments")
		return 2
	}
	file, err := os.Open(*policyPath)
	if err != nil {
		fmt.Fprintln(errOut, "listener-check: open policy:", err)
		return 1
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		fmt.Fprintln(errOut, "listener-check: policy must be a regular file")
		return 1
	}
	policy, err := listenerpolicy.Decode(file)
	if err != nil {
		fmt.Fprintln(errOut, "listener-check:", err)
		return 1
	}
	observed, err := snapshot()
	if err != nil {
		fmt.Fprintln(errOut, "listener-check: snapshot:", err)
		return 1
	}
	evaluation, err := listenerpolicy.Evaluate(policy, observed)
	if err != nil {
		fmt.Fprintln(errOut, "listener-check: evaluate:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(evaluation); err != nil {
		fmt.Fprintln(errOut, "listener-check: encode result:", err)
		return 1
	}
	return 0
}
