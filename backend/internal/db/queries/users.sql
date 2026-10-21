-- name: CreateUser :one
INSERT INTO users (username, email, display_name, password_hash, created_at, updated_at)
VALUES (@username, @email, @display_name, @password_hash, @now, @now)
RETURNING *;

-- name: GetUserByIDForUpdate :one
-- Trava a linha: a checagem "alvo é admin" não pode ficar velha até a escrita.
SELECT * FROM users WHERE id = @id FOR UPDATE;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = @email;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = COALESCE(email_verified_at, @now::timestamptz), updated_at = @now
WHERE id = @id;

-- name: SetPasswordHash :exec
UPDATE users SET password_hash = @password_hash, updated_at = @now WHERE id = @id;

-- name: CompletePasswordReset :exec
UPDATE users
SET password_hash = @password_hash,
    must_reset_password = false,
    email_verified_at = COALESCE(email_verified_at, @now::timestamptz),
    updated_at = @now
WHERE id = @id;

-- name: UpdateDisplayName :one
UPDATE users SET display_name = @display_name, updated_at = @now WHERE id = @id
RETURNING *;

-- name: SearchDirectory :many
SELECT username, display_name FROM users
WHERE disabled_at IS NULL AND email_verified_at IS NOT NULL
  AND (@query::text = '' OR username LIKE '%' || @query::text || '%' ESCAPE '\'
                         OR lower(display_name) LIKE '%' || @query::text || '%' ESCAPE '\')
  AND username > @after::text
ORDER BY username
LIMIT @lim::integer;
