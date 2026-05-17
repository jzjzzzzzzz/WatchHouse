CREATE TABLE control_findings (
    finding_id char(64) PRIMARY KEY CHECK (finding_id ~ '^[a-f0-9]{64}$'),
    host_id text NOT NULL CHECK (host_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'),
    rule_id text NOT NULL,
    observed_at timestamptz NOT NULL,
    priority text NOT NULL,
    summary text NOT NULL,
    rule_config jsonb NOT NULL,
    evidence_event_ids jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    last_evaluated_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    CHECK (jsonb_typeof(evidence_event_ids) = 'array')
);
CREATE INDEX control_findings_host_observed_idx ON control_findings (host_id, observed_at DESC, finding_id);
