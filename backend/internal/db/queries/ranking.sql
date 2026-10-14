-- name: Ranking :many
SELECT u.username, u.display_name, p.matches_played, p.wins, p.losses
FROM player_stats p
JOIN users u ON u.id = p.user_id
WHERE p.matches_played >= @min_matches::integer AND u.disabled_at IS NULL
ORDER BY p.wins DESC, (p.wins::float8 / p.matches_played) DESC, p.matches_played DESC, u.username
LIMIT @lim::integer;

-- name: GetPlayerStats :one
SELECT * FROM player_stats WHERE user_id = @user_id;

-- name: ListUserMatches :many
SELECT m.id, m.status, m.winner_team, m.score_a, m.score_b, m.created_at, m.finished_at, mp.seat
FROM match_players mp
JOIN matches m ON m.id = mp.match_id
WHERE mp.user_id = @user_id
  AND m.status <> 'in_progress'
  AND (sqlc.narg(cursor_finished_at)::timestamptz IS NULL
       OR (m.finished_at, m.id) < (sqlc.narg(cursor_finished_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY m.finished_at DESC, m.id DESC
LIMIT @lim::integer;
