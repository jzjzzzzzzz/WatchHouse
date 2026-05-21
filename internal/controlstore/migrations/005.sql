CREATE TABLE control_query_audit (
    audit_sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    decided_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    principal_id text NOT NULL,
    role text,
    resource text NOT NULL CHECK (resource IN ('events','findings')),
    host_id text NOT NULL CHECK (host_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'),
    decision text NOT NULL CHECK (decision IN ('allowed','denied'))
);
CREATE INDEX control_query_audit_principal_time_idx
    ON control_query_audit (principal_id, decided_at DESC);

CREATE FUNCTION reject_control_query_audit_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'control query audit rows are append-only';
END;
$$;
CREATE TRIGGER control_query_audit_append_only
BEFORE UPDATE OR DELETE ON control_query_audit
FOR EACH ROW EXECUTE FUNCTION reject_control_query_audit_mutation();
