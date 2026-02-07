CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";


CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ip_address          INET NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_request_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    hourly_request_count INTEGER NOT NULL DEFAULT 0,
    total_request_count  INTEGER NOT NULL DEFAULT 0,
    is_flagged          BOOLEAN NOT NULL DEFAULT FALSE,

    CONSTRAINT uq_sessions_ip UNIQUE (ip_address)
);

CREATE INDEX idx_sessions_ip ON sessions (ip_address);
CREATE INDEX idx_sessions_flagged ON sessions (is_flagged) WHERE is_flagged = TRUE;

CREATE TYPE job_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed'
);

CREATE TYPE job_operation AS ENUM (
    'image_convert',
    'image_compress',
    'image_remove_bg',
    'pdf_compress',
    'audio_convert',
    'audio_compress',
    'video_compress'
);

CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    operation       job_operation NOT NULL,
    status          job_status NOT NULL DEFAULT 'pending',

    input_filename  TEXT NOT NULL,
    output_filename TEXT,
    input_size      BIGINT NOT NULL DEFAULT 0,
    output_size     BIGINT,
    original_name   TEXT NOT NULL,

    params          JSONB NOT NULL DEFAULT '{}',

    file_nonce      BYTEA,

    error_message   TEXT,
    retry_count     INTEGER NOT NULL DEFAULT 0,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),

    CONSTRAINT chk_input_size CHECK (input_size >= 0),
    CONSTRAINT chk_retry_count CHECK (retry_count >= 0)
);

CREATE INDEX idx_jobs_session ON jobs (session_id);
CREATE INDEX idx_jobs_status ON jobs (status);
CREATE INDEX idx_jobs_expires ON jobs (expires_at) WHERE status != 'failed';
CREATE INDEX idx_jobs_created ON jobs (created_at);
CREATE INDEX idx_jobs_status_created ON jobs (status, created_at);


CREATE OR REPLACE FUNCTION reset_hourly_counts()
RETURNS INTEGER AS $$
DECLARE
    affected INTEGER;
BEGIN
    UPDATE sessions
    SET hourly_request_count = 0
    WHERE hourly_request_count > 0
      AND last_request_at < NOW() - INTERVAL '1 hour';
    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION cleanup_expired_jobs()
RETURNS TABLE(job_id UUID, input_file TEXT, output_file TEXT) AS $$
BEGIN
    RETURN QUERY
    DELETE FROM jobs
    WHERE expires_at < NOW()
    RETURNING id, input_filename, output_filename;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE VIEW admin_stats AS
SELECT
    (SELECT COUNT(*) FROM jobs WHERE status = 'pending') AS queue_length,
    (SELECT COUNT(*) FROM jobs WHERE status = 'processing') AS active_jobs,
    (SELECT COUNT(*) FROM jobs WHERE status = 'completed'
        AND completed_at > NOW() - INTERVAL '24 hours') AS completed_24h,
    (SELECT COUNT(*) FROM jobs WHERE status = 'failed'
        AND created_at > NOW() - INTERVAL '24 hours') AS failed_24h,
    (SELECT COUNT(*) FROM sessions
        WHERE last_request_at > NOW() - INTERVAL '1 hour') AS active_sessions;