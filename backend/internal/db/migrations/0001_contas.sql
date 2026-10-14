-- +goose Up
CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    username            text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_.]{3,20}$'),
    email               text NOT NULL UNIQUE CHECK (email = lower(email) AND position('@' IN email) > 1),
    display_name        text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 30),
    password_hash       text NOT NULL,
    is_admin            boolean NOT NULL DEFAULT false,
    email_verified_at   timestamptz,
    disabled_at         timestamptz,
    must_reset_password boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id             uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash          bytea NOT NULL UNIQUE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_seen_at        timestamptz NOT NULL,
    expires_at          timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    user_agent          text NOT NULL DEFAULT '',
    ip                  inet
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE user_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('verify_email', 'password_reset')),
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
CREATE INDEX user_tokens_user_purpose_idx ON user_tokens (user_id, purpose);

CREATE TABLE allowed_email_domains (
    domain     text PRIMARY KEY CHECK (domain = lower(domain) AND position('@' IN domain) = 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE allowed_email_domains;
DROP TABLE user_tokens;
DROP TABLE sessions;
DROP TABLE users;
