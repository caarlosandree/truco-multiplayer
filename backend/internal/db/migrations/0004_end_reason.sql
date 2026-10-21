-- +goose Up
ALTER TABLE matches ADD COLUMN end_reason text
    CONSTRAINT matches_end_reason_domain CHECK (end_reason IN ('game_over', 'forfeit', 'aborted', 'abandoned'));

-- Backfill, nesta ordem. O texto livre abandon_reason nunca é usado: o aborto de admin vem da auditoria.
UPDATE matches SET end_reason = 'forfeit'
WHERE status = 'finished' AND EXISTS (
    SELECT 1 FROM match_actions a
    WHERE a.match_id = matches.id AND a.source = 'system' AND a.action->>'kind' = 'sys_forfeit');
UPDATE matches SET end_reason = 'game_over' WHERE status = 'finished' AND end_reason IS NULL;
UPDATE matches SET end_reason = 'aborted'
WHERE status = 'abandoned' AND EXISTS (
    SELECT 1 FROM audit_log l WHERE l.action = 'match.abort' AND l.target_id = matches.id::text);
UPDATE matches SET end_reason = 'abandoned' WHERE status = 'abandoned' AND end_reason IS NULL;

ALTER TABLE matches ADD CONSTRAINT matches_end_reason_check CHECK ((status = 'in_progress') = (end_reason IS NULL));

-- +goose Down
ALTER TABLE matches DROP CONSTRAINT matches_end_reason_check;
ALTER TABLE matches DROP COLUMN end_reason;
