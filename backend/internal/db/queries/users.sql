-- name: CreateUser :one
INSERT INTO users (username, email, display_name, password_hash, created_at, updated_at)
VALUES (@username, @email, @display_name, @password_hash, @now, @now)
RETURNING *;

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
