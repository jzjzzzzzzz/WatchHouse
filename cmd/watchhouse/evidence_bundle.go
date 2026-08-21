package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/evidencebundle"
)

func runEvidenceBundle(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "evidence-bundle requires create or verify")
		return 2
	}
	switch args[0] {
	case "create":
		return runEvidenceCreate(args[1:], out, errOut)
	case "verify":
		return runEvidenceVerify(args[1:], out, errOut)
	default:
		fmt.Fprintln(errOut, "evidence-bundle requires create or verify")
		return 2
	}
}

func runEvidenceCreate(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("evidence-bundle create", flag.ContinueOnError)
	flags.SetOutput(errOut)
	input := flags.String("input", "", "absolute directory containing top-level JSON evidence")
	output := flags.String("output", "", "absolute new .tar.gz path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *input == "" || *output == "" {
		fmt.Fprintln(errOut, "--input and --output are required")
		return 2
	}
	manifest, err := evidencebundle.Create(*input, *output, time.Now())
	if err != nil {
		fmt.Fprintln(errOut, "evidence-bundle create:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(manifest); err != nil {
		return 1
	}
	return 0
}

func runEvidenceVerify(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("evidence-bundle verify", flag.ContinueOnError)
	flags.SetOutput(errOut)
	archive := flags.String("archive", "", "evidence archive path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *archive == "" {
		fmt.Fprintln(errOut, "--archive is required")
		return 2
	}
	manifest, err := evidencebundle.Verify(*archive)
	if err != nil {
		fmt.Fprintln(errOut, "evidence-bundle verify:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(manifest); err != nil {
		return 1
	}
	return 0
}
