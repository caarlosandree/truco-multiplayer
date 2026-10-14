-- name: InsertMatchPlayerStats :exec
INSERT INTO match_player_stats (
    match_id, user_id, team, won, points_for, points_against, hands_played, hands_won,
    raises_called, raises_folded, raises_won_by_fold, eleven_hands, eleven_runs, eleven_won,
    iron_hands, iron_won, punished, timeouts
) VALUES (
    @match_id, @user_id, @team, @won, @points_for, @points_against, @hands_played, @hands_won,
    @raises_called, @raises_folded, @raises_won_by_fold, @eleven_hands, @eleven_runs, @eleven_won,
    @iron_hands, @iron_won, @punished, @timeouts
);

-- name: AddPlayerStats :exec
INSERT INTO player_stats AS p (
    user_id, matches_played, wins, losses, points_for, points_against, hands_played, hands_won,
    raises_called, raises_folded, raises_won_by_fold, eleven_hands, eleven_runs, eleven_won,
    iron_hands, iron_won, punished, timeouts, updated_at
) VALUES (
    @user_id, 1, @wins, @losses, @points_for, @points_against, @hands_played, @hands_won,
    @raises_called, @raises_folded, @raises_won_by_fold, @eleven_hands, @eleven_runs, @eleven_won,
    @iron_hands, @iron_won, @punished, @timeouts, @now
)
ON CONFLICT (user_id) DO UPDATE SET
    matches_played     = p.matches_played + 1,
    wins               = p.wins + EXCLUDED.wins,
    losses             = p.losses + EXCLUDED.losses,
    points_for         = p.points_for + EXCLUDED.points_for,
    points_against     = p.points_against + EXCLUDED.points_against,
    hands_played       = p.hands_played + EXCLUDED.hands_played,
    hands_won          = p.hands_won + EXCLUDED.hands_won,
    raises_called      = p.raises_called + EXCLUDED.raises_called,
    raises_folded      = p.raises_folded + EXCLUDED.raises_folded,
    raises_won_by_fold = p.raises_won_by_fold + EXCLUDED.raises_won_by_fold,
    eleven_hands       = p.eleven_hands + EXCLUDED.eleven_hands,
    eleven_runs        = p.eleven_runs + EXCLUDED.eleven_runs,
    eleven_won         = p.eleven_won + EXCLUDED.eleven_won,
    iron_hands         = p.iron_hands + EXCLUDED.iron_hands,
    iron_won           = p.iron_won + EXCLUDED.iron_won,
    punished           = p.punished + EXCLUDED.punished,
    timeouts           = p.timeouts + EXCLUDED.timeouts,
    updated_at         = EXCLUDED.updated_at;

-- name: DeleteAllMatchPlayerStats :exec
DELETE FROM match_player_stats;

-- name: DeleteAllPlayerStats :exec
DELETE FROM player_stats;
