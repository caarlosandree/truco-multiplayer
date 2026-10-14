-- +goose Up
CREATE TABLE matches (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    seed           bigint NOT NULL,
    first_dealer   smallint NOT NULL CHECK (first_dealer BETWEEN 0 AND 3),
    engine_version integer NOT NULL,
    status         text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'finished', 'abandoned')),
    winner_team    smallint CHECK (winner_team IN (0, 1)),
    score_a        smallint,
    score_b        smallint,
    created_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    abandon_reason text,
    CONSTRAINT matches_finished_at_check CHECK ((status = 'in_progress') = (finished_at IS NULL)),
    CONSTRAINT matches_result_check CHECK (status <> 'finished' OR (winner_team IS NOT NULL AND score_a IS NOT NULL AND score_b IS NOT NULL)),
    CONSTRAINT matches_score_check CHECK (score_a BETWEEN 0 AND 12 AND score_b BETWEEN 0 AND 12)
);
CREATE INDEX matches_in_progress_idx ON matches (created_at) WHERE status = 'in_progress';
CREATE INDEX matches_closed_idx ON matches (finished_at DESC, id DESC) WHERE status <> 'in_progress';

CREATE TABLE match_players (
    match_id     uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    seat         smallint NOT NULL CHECK (seat BETWEEN 0 AND 3),
    user_id      uuid NOT NULL REFERENCES users (id),
    display_name text NOT NULL,
    PRIMARY KEY (match_id, seat),
    UNIQUE (match_id, user_id)
);
CREATE INDEX match_players_user_idx ON match_players (user_id);

CREATE TABLE match_actions (
    match_id   uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    ply        integer NOT NULL CHECK (ply >= 0),
    action     jsonb NOT NULL,
    source     text NOT NULL CHECK (source IN ('player', 'timeout', 'system')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (match_id, ply)
);

CREATE TABLE match_player_stats (
    match_id           uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    user_id            uuid NOT NULL REFERENCES users (id),
    team               smallint NOT NULL CHECK (team IN (0, 1)),
    won                boolean NOT NULL,
    points_for         integer NOT NULL DEFAULT 0,
    points_against     integer NOT NULL DEFAULT 0,
    hands_played       integer NOT NULL DEFAULT 0,
    hands_won          integer NOT NULL DEFAULT 0,
    raises_called      integer NOT NULL DEFAULT 0,
    raises_folded      integer NOT NULL DEFAULT 0,
    raises_won_by_fold integer NOT NULL DEFAULT 0,
    eleven_hands       integer NOT NULL DEFAULT 0,
    eleven_runs        integer NOT NULL DEFAULT 0,
    eleven_won         integer NOT NULL DEFAULT 0,
    iron_hands         integer NOT NULL DEFAULT 0,
    iron_won           integer NOT NULL DEFAULT 0,
    punished           integer NOT NULL DEFAULT 0,
    timeouts           integer NOT NULL DEFAULT 0,
    PRIMARY KEY (match_id, user_id)
);

CREATE TABLE player_stats (
    user_id            uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    matches_played     integer NOT NULL DEFAULT 0,
    wins               integer NOT NULL DEFAULT 0,
    losses             integer NOT NULL DEFAULT 0,
    points_for         integer NOT NULL DEFAULT 0,
    points_against     integer NOT NULL DEFAULT 0,
    hands_played       integer NOT NULL DEFAULT 0,
    hands_won          integer NOT NULL DEFAULT 0,
    raises_called      integer NOT NULL DEFAULT 0,
    raises_folded      integer NOT NULL DEFAULT 0,
    raises_won_by_fold integer NOT NULL DEFAULT 0,
    eleven_hands       integer NOT NULL DEFAULT 0,
    eleven_runs        integer NOT NULL DEFAULT 0,
    eleven_won         integer NOT NULL DEFAULT 0,
    iron_hands         integer NOT NULL DEFAULT 0,
    iron_won           integer NOT NULL DEFAULT 0,
    punished           integer NOT NULL DEFAULT 0,
    timeouts           integer NOT NULL DEFAULT 0,
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE player_stats;
DROP TABLE match_player_stats;
DROP TABLE match_actions;
DROP TABLE match_players;
DROP TABLE matches;
