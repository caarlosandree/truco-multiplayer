-- name: InsertAudit :exec
INSERT INTO audit_log (actor_id, action, target_type, target_id, details, created_at)
VALUES (@actor_id, @action, @target_type, @target_id, @details, @now);

-- name: ListAudit :many
SELECT a.id, a.actor_id, u.username AS actor_username, a.action, a.target_type, a.target_id, a.details, a.created_at
FROM audit_log a
JOIN users u ON u.id = a.actor_id
WHERE sqlc.narg(cursor_created_at)::timestamptz IS NULL
   OR (a.created_at, a.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
ORDER BY a.created_at DESC, a.id DESC
LIMIT @lim::integer;

-- name: SearchUsers :many
SELECT * FROM users
WHERE kind = 'human'
  AND (@query::text = '' OR username LIKE '%' || @query::text || '%' ESCAPE '\'
                         OR email LIKE '%' || @query::text || '%' ESCAPE '\')
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) > (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at, id
LIMIT @lim::integer;

-- name: SetUserDisabled :execrows
UPDATE users SET disabled_at = sqlc.narg(disabled_at), updated_at = @now WHERE id = @id;

-- name: SetUserAdmin :execrows
UPDATE users SET is_admin = @is_admin, updated_at = @now WHERE id = @id;

-- name: SetMustResetPassword :execrows
UPDATE users SET must_reset_password = true, updated_at = @now WHERE id = @id;

-- name: CountActiveAdmins :one
SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL;

-- name: LockAdmins :exec
SELECT id FROM users WHERE is_admin FOR UPDATE;
