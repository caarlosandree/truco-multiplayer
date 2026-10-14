-- name: DeleteUnusedTokens :exec
DELETE FROM user_tokens WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL;

-- name: CreateUserToken :exec
INSERT INTO user_tokens (user_id, purpose, token_hash, created_at, expires_at)
VALUES (@user_id, @purpose, @token_hash, @now, @expires_at);

-- name: GetValidUserToken :one
SELECT user_id FROM user_tokens
WHERE token_hash = @token_hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now;

-- name: ConsumeUserToken :one
UPDATE user_tokens SET used_at = @now::timestamptz
WHERE token_hash = @token_hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now
RETURNING user_id;

-- name: DeleteStaleTokens :execrows
DELETE FROM user_tokens WHERE expires_at <= @now OR used_at <= @used_before::timestamptz;
