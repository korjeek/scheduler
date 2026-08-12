-- +goose Up
-- +goose StatementBegin

CREATE TYPE task_status AS ENUM ('pending', 'running', 'success', 'failed');

CREATE TABLE IF NOT EXISTS tasks (
                                     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                     name            TEXT NOT NULL,
                                     queue_name      TEXT NOT NULL,
                                     cron            TEXT NOT NULL,
                                     payload         BYTEA NOT NULL,
                                     next_run_at     TIMESTAMP WITH TIME ZONE,
                                     status          task_status NOT NULL DEFAULT 'pending'::task_status,
                                     last_run_at     TIMESTAMP WITH TIME ZONE,
                                     last_error      TEXT,
                                     retry_count     INT DEFAULT 0,
                                     max_retries     INT DEFAULT 3,
                                     worker_id       TEXT,
                                     locked_until    TIMESTAMP WITH TIME ZONE,
                                     created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                     updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_poll
    ON tasks (queue_name, next_run_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status);
CREATE INDEX IF NOT EXISTS idx_tasks_locked_until ON tasks (locked_until);

CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS tasks;
DROP TYPE IF EXISTS task_status;
-- +goose StatementEnd
