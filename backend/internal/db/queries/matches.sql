-- name: CreateMatch :one
INSERT INTO matches (seed, first_dealer, engine_version, created_at, ranked)
VALUES (@seed, @first_dealer, @engine_version, @now, @ranked)
RETURNING id;

-- name: AddMatchPlayer :exec
INSERT INTO match_players (match_id, seat, user_id, display_name)
VALUES (@match_id, @seat, @user_id, @display_name);

-- name: LockMatchStatus :one
-- FOR SHARE serializa com Abandon (UPDATE) e Finish (FOR UPDATE).
SELECT status FROM matches WHERE id = @id FOR SHARE;

-- name: NextPly :one
SELECT coalesce(max(ply) + 1, 0)::integer FROM match_actions WHERE match_id = @match_id;

-- name: InsertMatchAction :exec
INSERT INTO match_actions (match_id, ply, action, source, created_at)
VALUES (@match_id::uuid, @ply::integer, @action::jsonb, @source::text, @now::timestamptz);

-- name: GetMatchAction :one
SELECT action, source FROM match_actions WHERE match_id = @match_id AND ply = @ply;

-- name: GetMatch :one
SELECT * FROM matches WHERE id = @id;

-- name: GetMatchForUpdate :one
SELECT * FROM matches WHERE id = @id FOR UPDATE;

-- name: ListMatchPlayers :many
SELECT mp.seat, mp.user_id, mp.display_name, u.username, u.kind
FROM match_players mp
JOIN users u ON u.id = mp.user_id
WHERE mp.match_id = @match_id
ORDER BY mp.seat;

-- name: ListMatchActions :many
SELECT ply, action, source FROM match_actions WHERE match_id = @match_id ORDER BY ply;

-- name: ListInProgressMatchIDs :many
SELECT id FROM matches WHERE status = 'in_progress' ORDER BY created_at, id;

-- name: ListFinishedMatchIDs :many
SELECT id FROM matches WHERE status = 'finished' AND ranked ORDER BY finished_at, id;

-- name: FinishMatch :one
-- ranked final = o que a partida já tinha E o que a sala informa: nunca "re-ranqueia".
UPDATE matches
SET ranked = ranked AND @ranked::boolean, status = 'finished', winner_team = @winner_team::smallint, score_a = @score_a::smallint,
    score_b = @score_b::smallint, finished_at = @now::timestamptz, end_reason = @end_reason::text
WHERE id = @id
RETURNING ranked;

-- name: AbandonMatch :execrows
UPDATE matches
SET status = 'abandoned', finished_at = @now::timestamptz, abandon_reason = @reason::text,
    end_reason = @end_reason::text
WHERE id = @id AND status = 'in_progress';

-- name: ListInProgressMatches :many
-- Mais antigas primeiro; cursor por (created_at, id).
SELECT id, created_at FROM matches
WHERE status = 'in_progress'
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (created_at, id) > (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at, id
LIMIT @lim::integer;
