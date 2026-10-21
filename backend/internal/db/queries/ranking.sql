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
SELECT m.id, m.status, m.winner_team, m.score_a, m.score_b, m.created_at, m.finished_at, m.end_reason, mp.seat
FROM match_players mp
JOIN matches m ON m.id = mp.match_id
WHERE mp.user_id = @user_id
  AND m.status <> 'in_progress'
  AND (sqlc.narg(cursor_finished_at)::timestamptz IS NULL
       OR (m.finished_at, m.id) < (sqlc.narg(cursor_finished_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY m.finished_at DESC, m.id DESC
LIMIT @lim::integer;

-- name: ListPlayersForMatches :many
SELECT mp.match_id, mp.seat, u.username, mp.display_name
FROM match_players mp
JOIN users u ON u.id = mp.user_id
WHERE mp.match_id = ANY(@ids::uuid[])
ORDER BY mp.match_id, mp.seat;

-- name: ListPartners :many
SELECT u.username, u.display_name, count(*)::integer AS matches,
       count(*) FILTER (WHERE s.won)::integer AS wins
FROM match_players me
JOIN match_players p ON p.match_id = me.match_id AND p.seat = (me.seat + 2) % 4
JOIN matches m ON m.id = me.match_id AND m.status = 'finished'
JOIN match_player_stats s ON s.match_id = me.match_id AND s.user_id = me.user_id
JOIN users u ON u.id = p.user_id AND u.disabled_at IS NULL
WHERE me.user_id = @user_id
GROUP BY u.username, u.display_name
ORDER BY matches DESC, wins DESC, u.username
LIMIT @lim::integer;
