-- Create enum type for quote status
DO $$ BEGIN
    CREATE TYPE quotes_status AS ENUM ('PENDING', 'RUNNING', 'SUCCESS', 'FAILED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create quotes table
CREATE TABLE IF NOT EXISTS quotes
(
    id           UUID PRIMARY KEY,
    base         CHAR(3) NOT NULL,
    quote        CHAR(3) NOT NULL,
    price        NUMERIC(18,6),
    status       quotes_status NOT NULL,
    error        TEXT,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ
);

-- Index for finding latest quotes by pair
CREATE INDEX IF NOT EXISTS idx_quotes_pair_time
    ON quotes(base, quote, updated_at DESC);

-- Partial unique index for deduplication of in-flight updates
CREATE UNIQUE INDEX IF NOT EXISTS uniq_quotes_pair_pending
    ON quotes(base, quote)
    WHERE status IN ('PENDING','RUNNING');
