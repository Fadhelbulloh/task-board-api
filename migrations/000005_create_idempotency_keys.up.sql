-- One row per (user, Idempotency-Key). The row is written in the same
-- transaction as the task it belongs to (see TaskService.Create), so a
-- committed row always has its response_body, and a rolled-back request
-- leaves nothing behind.
CREATE TABLE idempotency_keys (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key           UUID NOT NULL,
    request_hash  TEXT NOT NULL,
    response_body BYTEA,             -- filled before commit; raw bytes so the replay is byte-identical
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, key)
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);
