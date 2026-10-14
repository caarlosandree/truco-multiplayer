-- +goose Up
CREATE TABLE audit_log (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    actor_id    uuid NOT NULL REFERENCES users (id),
    action      text NOT NULL,
    target_type text NOT NULL,
    target_id   text NOT NULL,
    details     jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC, id DESC);

-- +goose Down
DROP TABLE audit_log;
