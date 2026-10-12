CREATE TYPE task_status AS ENUM ('todo', 'in_progress', 'done');

CREATE TABLE tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id),
    assignee_id UUID REFERENCES users(id),
    team_id     UUID REFERENCES teams(id) ON DELETE SET NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      task_status NOT NULL DEFAULT 'todo',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GET /tasks filters by owner OR assignee and sorts by created_at desc.
-- Postgres can BitmapOr these two.
CREATE INDEX tasks_owner_created_idx    ON tasks (owner_id, created_at DESC);
CREATE INDEX tasks_assignee_created_idx ON tasks (assignee_id, created_at DESC) WHERE assignee_id IS NOT NULL;

-- Prefix search on title: lower(title) LIKE 'term%'.
CREATE INDEX tasks_title_lower_idx ON tasks (lower(title) text_pattern_ops);
