CREATE TABLE control_events (
    host_id text NOT NULL CHECK (host_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'),
    event_id char(64) NOT NULL CHECK (event_id ~ '^[a-f0-9]{64}$'),
    schema_version integer NOT NULL CHECK (schema_version = 1),
    observed_at timestamptz NOT NULL,
    agent_received_at timestamptz NOT NULL,
    kind text NOT NULL CHECK (kind = 'ssh.authentication'),
    payload jsonb NOT NULL,
    content_digest char(64) NOT NULL CHECK (content_digest ~ '^[a-f0-9]{64}$'),
    ingested_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    PRIMARY KEY (host_id, event_id)
);
CREATE INDEX control_events_host_observed_idx ON control_events (host_id, observed_at, event_id);
