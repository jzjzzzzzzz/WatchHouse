CREATE TABLE control_probe_observations (
    observation_id char(64) PRIMARY KEY CHECK (observation_id ~ '^[a-f0-9]{64}$'),
    probe_id text NOT NULL CHECK (probe_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'),
    observed_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    target_url text NOT NULL,
    connected_address text NOT NULL,
    http_status integer NOT NULL CHECK (http_status BETWEEN 100 AND 599),
    expected boolean NOT NULL,
    content_digest char(64) NOT NULL CHECK (content_digest ~ '^[a-f0-9]{64}$'),
    payload jsonb NOT NULL
);
CREATE INDEX control_probe_observations_probe_time_idx
    ON control_probe_observations (probe_id, observed_at DESC, observation_id);
