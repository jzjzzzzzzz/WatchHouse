ALTER TABLE control_findings
    ADD COLUMN finding_sequence bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
CREATE INDEX control_findings_host_sequence_idx
    ON control_findings (host_id, finding_sequence DESC);
