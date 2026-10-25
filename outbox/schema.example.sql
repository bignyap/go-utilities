-- Transactional Outbox Table Schema and Trigger
-- Designed for PostgreSQL 14+ with jackc/pgx/v5

CREATE TABLE IF NOT EXISTS outbox_events (
    id            BIGSERIAL PRIMARY KEY,
    stream_name   VARCHAR(128) NOT NULL,
    event_key     VARCHAR(128) NOT NULL DEFAULT '',
    event_type    VARCHAR(128) NOT NULL,
    payload       JSONB NOT NULL,
    headers       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at  TIMESTAMPTZ,
    retry_count   INT NOT NULL DEFAULT 0,
    last_error    TEXT
);

-- Partial index for high-speed queries on unprocessed events
CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed 
ON outbox_events (id ASC) 
WHERE processed_at IS NULL;

-- Statement-level notification trigger for sub-5ms push latency
CREATE OR REPLACE FUNCTION notify_outbox_event() 
RETURNS TRIGGER AS $$
BEGIN
    -- Send lightweight doorbell signal with empty payload (avoid 8KB pg_notify limit)
    PERFORM pg_notify('outbox_events_channel', '');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_outbox_notify ON outbox_events;
CREATE TRIGGER trg_outbox_notify
AFTER INSERT ON outbox_events
FOR EACH STATEMENT
EXECUTE FUNCTION notify_outbox_event();

