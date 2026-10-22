-- +goose Up
ALTER TABLE users ADD COLUMN kind text NOT NULL DEFAULT 'human' CONSTRAINT users_kind_check CHECK (kind IN ('human', 'bot'));
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_human_credentials
    CHECK (kind = 'bot' OR (email IS NOT NULL AND password_hash IS NOT NULL));

INSERT INTO users (username, display_name, kind, email_verified_at) VALUES
    ('bot_seu_ze', 'Seu Zé', 'bot', NULL),
    ('bot_dona_cida', 'Dona Cida', 'bot', NULL),
    ('bot_tio_juca', 'Tio Juca', 'bot', NULL),
    ('bot_vo_nena', 'Vó Nena', 'bot', NULL),
    ('bot_compadre_tiao', 'Compadre Tião', 'bot', NULL),
    ('bot_dona_lurdes', 'Dona Lurdes', 'bot', NULL),
    ('bot_primo_dede', 'Primo Dedé', 'bot', NULL),
    ('bot_tia_fia', 'Tia Fia', 'bot', NULL);

ALTER TABLE matches ADD COLUMN ranked boolean NOT NULL DEFAULT true;

ALTER TABLE match_actions DROP CONSTRAINT match_actions_source_check;
ALTER TABLE match_actions ADD CONSTRAINT match_actions_source_check
    CHECK (source IN ('player', 'timeout', 'system', 'bot'));

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM match_players mp JOIN users u ON u.id = mp.user_id WHERE u.kind = 'bot') THEN
        RAISE EXCEPTION 'não é possível reverter 0005: há partidas com bots em match_players';
    END IF;
    IF EXISTS (SELECT 1 FROM match_actions WHERE source = 'bot') THEN
        RAISE EXCEPTION 'não é possível reverter 0005: há ações com source = bot em match_actions';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE match_actions DROP CONSTRAINT match_actions_source_check;
ALTER TABLE match_actions ADD CONSTRAINT match_actions_source_check
    CHECK (source IN ('player', 'timeout', 'system'));
ALTER TABLE matches DROP COLUMN ranked;
DELETE FROM users WHERE kind = 'bot';
ALTER TABLE users DROP CONSTRAINT users_human_credentials;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP COLUMN kind;
