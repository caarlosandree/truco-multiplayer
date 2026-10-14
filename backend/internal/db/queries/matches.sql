-- name: CreateMatch :one
INSERT INTO matches (seed, first_dealer, engine_version, created_at)
VALUES (@seed, @first_dealer, @engine_version, @now)
RETURNING id;

-- name: AddMatchPlayer :exec
INSERT INTO match_players (match_id, seat, user_id, display_name)
VALUES (@match_id, @seat, @user_id, @display_name);

-- name: AppendMatchAction :execrows
INSERT INTO match_actions (match_id, ply, action, source, created_at)
SELECT @match_id::uuid, @ply::integer, @action::jsonb, @source::text, @now::timestamptz
WHERE EXISTS (SELECT 1 FROM matches WHERE id = @match_id::uuid AND status = 'in_progress');

-- name: GetMatch :one
SELECT * FROM matches WHERE id = @id;

-- name: GetMatchForUpdate :one
SELECT * FROM matches WHERE id = @id FOR UPDATE;

-- name: ListMatchPlayers :many
SELECT mp.seat, mp.user_id, mp.display_name, u.username
FROM match_players mp
JOIN users u ON u.id = mp.user_id
WHERE mp.match_id = @match_id
ORDER BY mp.seat;

-- name: ListMatchActions :many
SELECT ply, action, source FROM match_actions WHERE match_id = @match_id ORDER BY ply;

-- name: ListInProgressMatchIDs :many
SELECT id FROM matches WHERE status = 'in_progress' ORDER BY created_at, id;

-- name: ListFinishedMatchIDs :many
SELECT id FROM matches WHERE status = 'finished' ORDER BY finished_at, id;

-- name: FinishMatch :exec
UPDATE matches
SET status = 'finished', winner_team = @winner_team::smallint, score_a = @score_a::smallint,
    score_b = @score_b::smallint, finished_at = @now::timestamptz
WHERE id = @id;

-- name: AbandonMatch :execrows
UPDATE matches
SET status = 'abandoned', finished_at = @now::timestamptz, abandon_reason = @reason::text
WHERE id = @id AND status = 'in_progress';
