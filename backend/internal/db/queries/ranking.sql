-- name: Ranking :many
-- by_winrate: aproveitamento → vitórias → partidas → username; senão vitórias → aproveitamento → partidas → username.
WITH agg AS (
    SELECT u.id AS user_id, u.username, u.display_name,
           count(*)::integer AS matches_played,
           (count(*) FILTER (WHERE s.won))::integer AS wins
    FROM match_player_stats s
    JOIN matches m ON m.id = s.match_id AND m.status = 'finished' AND m.ranked
    JOIN users u ON u.id = s.user_id AND u.kind = 'human' AND u.disabled_at IS NULL
    WHERE sqlc.narg(since)::timestamptz IS NULL OR m.finished_at >= sqlc.narg(since)::timestamptz
    GROUP BY u.id, u.username, u.display_name
    HAVING count(*) >= @min_matches::integer
), ranked AS (
    SELECT agg.*, (agg.matches_played - agg.wins)::integer AS losses,
           (row_number() OVER (ORDER BY
               CASE WHEN @by_winrate::boolean THEN agg.wins::float8 / agg.matches_played ELSE agg.wins::float8 END DESC,
               CASE WHEN @by_winrate::boolean THEN agg.wins::float8 ELSE agg.wins::float8 / agg.matches_played END DESC,
               agg.matches_played DESC, agg.username))::integer AS pos
    FROM agg
)
SELECT user_id, username, display_name, matches_played, wins, losses, pos
FROM ranked
WHERE pos <= @lim::integer OR user_id = sqlc.narg(viewer)::uuid
ORDER BY pos;

-- name: GetPlayerStats :one
SELECT * FROM player_stats WHERE user_id = @user_id;

-- name: ListUserMatches :many
SELECT m.id, m.status, m.winner_team, m.score_a, m.score_b, m.created_at, m.finished_at, m.end_reason, m.ranked, mp.seat
FROM match_players mp
JOIN matches m ON m.id = mp.match_id
WHERE mp.user_id = @user_id
  AND m.status <> 'in_progress'
  AND (sqlc.narg(cursor_finished_at)::timestamptz IS NULL
       OR (m.finished_at, m.id) < (sqlc.narg(cursor_finished_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY m.finished_at DESC, m.id DESC
LIMIT @lim::integer;

-- name: ListPlayersForMatches :many
SELECT mp.match_id, mp.seat, u.username, mp.display_name, (u.kind = 'bot') AS bot
FROM match_players mp
JOIN users u ON u.id = mp.user_id
WHERE mp.match_id = ANY(@ids::uuid[])
ORDER BY mp.match_id, mp.seat;

-- name: ListPartners :many
SELECT u.username, u.display_name, count(*)::integer AS matches,
       count(*) FILTER (WHERE s.won)::integer AS wins
FROM match_players me
JOIN match_players p ON p.match_id = me.match_id AND p.seat = (me.seat + 2) % 4
JOIN matches m ON m.id = me.match_id AND m.status = 'finished' AND m.ranked
JOIN match_player_stats s ON s.match_id = me.match_id AND s.user_id = me.user_id
JOIN users u ON u.id = p.user_id AND u.kind = 'human' AND u.disabled_at IS NULL
WHERE me.user_id = @user_id
GROUP BY u.username, u.display_name
ORDER BY matches DESC, wins DESC, u.username
LIMIT @lim::integer;
