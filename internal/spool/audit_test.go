package spool

import (
	"context"
	"errors"
	"testing"
)

func TestAuditValidStateDoesNotConsumeOrRepair(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 3)
	before, _ := s.Stats(ctx)
	audit, err := s.Audit(ctx)
	after, _ := s.Stats(ctx)
	if err != nil || !audit.Valid || audit.Records != 3 || audit.Checkpoints != 1 || audit.PayloadBytes != before.PendingBytes || before != after {
		t.Fatalf("audit %+v %v", audit, err)
	}
}

func TestAuditDetectsCorruptIndexesAndContent(t *testing.T) {
	for _, mutation := range []string{
		"UPDATE events SET content_digest='bad'",
		"UPDATE events SET host_id='other-host'",
		"UPDATE events SET payload=X'7B7D',payload_bytes=2",
		"UPDATE queue_state SET pending_records=99 WHERE id=1",
		"UPDATE checkpoints SET source='unknown-source'",
		"UPDATE checkpoints SET cursor=''",
	} {
		t.Run(mutation, func(t *testing.T) {
			s, _ := openTest(t, DefaultOptions())
			appendEvents(t, s, 1)
			if _, err := s.db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			audit, err := s.Audit(context.Background())
			if !errors.Is(err, ErrCorrupt) || audit.Valid {
				t.Fatalf("corruption accepted %+v %v", audit, err)
			}
		})
	}
}

func TestAuditCanceledAndEmptyState(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := s.Audit(ctx); err == nil || result.Valid {
		t.Fatal("canceled audit valid")
	}
	if result, err := s.Audit(context.Background()); err != nil || !result.Valid || result.Records != 0 {
		t.Fatal("empty queue audit failed")
	}
}
