-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at, expires_at, absolute_expires_at, user_agent, ip)
VALUES (@user_id, @token_hash, @now, @now, @expires_at, @absolute_expires_at, @user_agent, NULLIF(@ip::text, '')::inet)
RETURNING id;

-- name: GetSessionPrincipal :one
SELECT s.id AS session_id, s.last_seen_at, u.id AS user_id, u.username, u.email, u.display_name, u.is_admin
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.expires_at > @now
  AND s.absolute_expires_at > @now
  AND u.disabled_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = @now, expires_at = LEAST(@expires_at::timestamptz, absolute_expires_at)
WHERE id = @id;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = @token_hash;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= @now OR absolute_expires_at <= @now;
