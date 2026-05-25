CREATE TABLE control_listener_snapshots (
    snapshot_id char(64) PRIMARY KEY CHECK (snapshot_id ~ '^[a-f0-9]{64}$'),
    host_id text NOT NULL CHECK (host_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'),
    boot_id uuid NOT NULL,
    network_namespace text NOT NULL CHECK (network_namespace ~ '^net:\[[0-9]+\]$'),
    observed_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    listener_count integer NOT NULL CHECK (listener_count >= 0 AND listener_count <= 16384),
    content_digest char(64) NOT NULL CHECK (content_digest ~ '^[a-f0-9]{64}$'),
    payload jsonb NOT NULL
);
CREATE INDEX control_listener_snapshots_host_time_idx
    ON control_listener_snapshots (host_id, observed_at DESC, snapshot_id);
