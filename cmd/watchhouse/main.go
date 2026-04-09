package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"watchhouse/internal/detection"
	"watchhouse/internal/replay"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(errOut, "Usage: watchhouse replay --host HOST [--input FILE|-] [--threshold 5] [--window 5m] [--max-events 8192]")
		return 0
	}
	if args[0] != "replay" {
		fmt.Fprintln(errOut, "unknown command; use --help")
		return 2
	}
	flags := flag.NewFlagSet("replay", flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "offline host label; not an authenticated identity")
	input := flags.String("input", "-", "journal JSON Lines file; - reads stdin")
	defaults := detection.DefaultConfig()
	threshold := flags.Int("threshold", defaults.Threshold, "failure count before success")
	window := flags.Duration("window", defaults.Window, "ordered event-time observation window")
	maxEvents := flags.Int("max-events", defaults.MaxEvents, "maximum in-memory dedup events")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return 2
	}
	if *input != "-" {
		file, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(errOut, "input:", err)
			return 1
		}
		defer file.Close()
		in = file
	}
	stats, err := replay.Run(in, out, *host, detection.Config{Window: *window, Threshold: *threshold, MaxEvents: *maxEvents}, time.Now)
	if encodeErr := json.NewEncoder(errOut).Encode(struct {
		Type  string       `json:"type"`
		Stats replay.Stats `json:"stats"`
	}{"summary", stats}); encodeErr != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(errOut, "replay:", err)
		return 1
	}
	return 0
}
