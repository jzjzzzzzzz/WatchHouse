package controlstore

import (
	"context"
	"fmt"

	"watchhouse/internal/detection"
	"watchhouse/internal/transport"
)

// Processor makes event acceptance and finding evaluation retry-safe. Events
// commit first; if evaluation fails the HTTP request receives no receipts, so a
// retry reuses the idempotent event rows and attempts evaluation again.
type Processor struct {
	store   *Store
	config  detection.Config
	maxScan int
}

func NewProcessor(store *Store, config detection.Config, maxScan int) (*Processor, error) {
	if store == nil || maxScan < 1 || maxScan > 100_000 {
		return nil, fmt.Errorf("invalid event processor configuration")
	}
	if _, err := detection.NewSSH(config); err != nil {
		return nil, err
	}
	return &Processor{store: store, config: config, maxScan: maxScan}, nil
}

func (processor *Processor) CommitBatch(ctx context.Context, host string, items []transport.Item) error {
	if err := processor.store.CommitBatch(ctx, host, items); err != nil {
		return err
	}
	_, err := processor.store.RunSSHDetection(ctx, host, processor.config, processor.maxScan)
	return err
}
