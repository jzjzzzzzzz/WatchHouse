ALTER TABLE control_events
    ADD COLUMN ingest_sequence bigint GENERATED ALWAYS AS IDENTITY;
ALTER TABLE control_events
    ADD CONSTRAINT control_events_ingest_sequence_unique UNIQUE (ingest_sequence);
CREATE INDEX control_events_host_sequence_idx ON control_events (host_id, ingest_sequence DESC);
