-- name: DomainAllowed :one
SELECT EXISTS (SELECT 1 FROM allowed_email_domains WHERE domain = @domain);

-- name: CountDomains :one
SELECT count(*) FROM allowed_email_domains;

-- name: InsertDomain :exec
INSERT INTO allowed_email_domains (domain, created_at, created_by)
VALUES (@domain, @now, sqlc.narg(created_by))
ON CONFLICT (domain) DO NOTHING;

-- name: ListDomains :many
SELECT domain, created_at, created_by FROM allowed_email_domains ORDER BY domain;

-- name: DeleteDomain :execrows
DELETE FROM allowed_email_domains WHERE domain = @domain;

-- name: LockDomains :exec
SELECT domain FROM allowed_email_domains FOR UPDATE;
