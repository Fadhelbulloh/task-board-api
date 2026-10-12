CREATE TABLE task_logs (
    id         BIGSERIAL PRIMARY KEY,
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    actor_id   UUID NOT NULL REFERENCES users(id),
    action     TEXT NOT NULL,
    old_value  JSONB,
    new_value  JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX task_logs_task_id_idx ON task_logs (task_id, created_at DESC);
