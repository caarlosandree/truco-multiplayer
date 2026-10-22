// Package admin implementa as operações administrativas, sempre auditadas na mesma transação.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// MatchAborter encerra a sala viva de uma partida abortada. A Spec 3 implementa.
type MatchAborter interface {
	Abort(ctx context.Context, matchID string) error
}

// SessionDropper derruba as conexões em tempo real de um usuário recém-desativado (o Lobby implementa).
type SessionDropper interface {
	DropUser(userID string)
}

type NoopDropper struct{}

func (NoopDropper) DropUser(string) {}

type NoopAborter struct{}

func (NoopAborter) Abort(context.Context, string) error { return nil }

var (
	ErrNotFound   = errors.New("admin: não encontrado")
	ErrSelfAction = errors.New("admin: não é possível aplicar esta ação a si mesmo")
	// ErrTargetIsAdmin: admin não age sobre outro admin; a regra mora no serviço, não no handler.
	ErrTargetIsAdmin = errors.New("admin: não é possível aplicar esta ação a outro administrador")
	// ErrNotAdmin: DemoteAdmin sobre quem não é administrador (nada é alterado nem auditado).
	ErrNotAdmin      = errors.New("admin: o usuário não é administrador")
	ErrLastAdmin     = errors.New("admin: é preciso manter ao menos um administrador ativo")
	ErrLastDomain    = errors.New("admin: é preciso manter ao menos um domínio permitido")
	ErrInvalidDomain = errors.New("admin: domínio inválido")
)

const pageSize = 50

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

type Options struct {
	Pool    *pgxpool.Pool
	Auth    *auth.Service
	Aborter MatchAborter
	Dropper SessionDropper
	Now     func() time.Time
	Logger  *slog.Logger
}

type Service struct {
	pool    *pgxpool.Pool
	q       *dbgen.Queries
	auth    *auth.Service
	aborter MatchAborter
	dropper SessionDropper
	now     func() time.Time
	log     *slog.Logger
}

func NewService(o Options) *Service {
	s := &Service{pool: o.Pool, q: dbgen.New(o.Pool), auth: o.Auth, aborter: o.Aborter, dropper: o.Dropper, now: o.Now, log: o.Logger}
	if s.aborter == nil {
		s.aborter = NoopAborter{}
	}
	if s.dropper == nil {
		s.dropper = NoopDropper{}
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	return s
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || db.IsInvalidInput(err) {
		return ErrNotFound
	}
	return err
}

func writeAudit(ctx context.Context, q *dbgen.Queries, actorID, action, targetType, targetID string, details map[string]any, now time.Time) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return q.InsertAudit(ctx, dbgen.InsertAuditParams{ActorID: actorID, Action: action, TargetType: targetType, TargetID: targetID, Details: raw, Now: now})
}

// audited executa fn e grava a auditoria na mesma transação; se fn falhar, nada é gravado.
func (s *Service) audited(ctx context.Context, actor auth.Principal, action, targetType, targetID string, details map[string]any, fn func(q *dbgen.Queries, now time.Time) error) error {
	now := s.now()
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := fn(q, now); err != nil {
			return err
		}
		return writeAudit(ctx, q, actor.UserID, action, targetType, targetID, details, now)
	})
}

func requireRow(n int64, err error) error {
	if err != nil {
		return notFound(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// refuseAdminTarget lê o alvo dentro da transação e recusa se for admin, antes de qualquer escrita.
func refuseAdminTarget(ctx context.Context, q *dbgen.Queries, userID string) error {
	target, err := q.GetUserByIDForUpdate(ctx, userID)
	if err != nil {
		return notFound(err)
	}
	if target.Kind == "bot" { // contas de bot não são administráveis: somem como inexistentes
		return ErrNotFound
	}
	if target.IsAdmin {
		return ErrTargetIsAdmin
	}
	return nil
}

func (s *Service) DisableUser(ctx context.Context, actor auth.Principal, userID string) error {
	if userID == actor.UserID {
		return ErrSelfAction
	}
	err := s.audited(ctx, actor, "user.disable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := refuseAdminTarget(ctx, q, userID); err != nil {
			return err
		}
		if err := requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: &now, Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
	if err != nil {
		return err
	}
	// Só depois do commit: a conexão cai com a conta já desativada de fato.
	s.dropper.DropUser(userID)
	return nil
}

func (s *Service) EnableUser(ctx context.Context, actor auth.Principal, userID string) error {
	return s.audited(ctx, actor, "user.enable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := refuseAdminTarget(ctx, q, userID); err != nil {
			return err
		}
		return requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: nil, Now: now, ID: userID}))
	})
}

// ForcePasswordReset marca a conta para redefinição, encerra suas sessões e audita na mesma
// transação. Falhas no envio do link, depois do commit, são apenas registradas em log.
func (s *Service) ForcePasswordReset(ctx context.Context, actor auth.Principal, userID string) error {
	err := s.audited(ctx, actor, "user.force_password_reset", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := refuseAdminTarget(ctx, q, userID); err != nil {
			return err
		}
		if err := requireRow(q.SetMustResetPassword(ctx, dbgen.SetMustResetPasswordParams{Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
	if err != nil {
		return err
	}
	// O estado já foi confirmado: uma falha ao emitir/enviar o link não desfaz a
	// ação, então é só registrada (o admin pode repetir a ação para reenviar).
	if err := s.auth.SendPasswordReset(ctx, userID, true); err != nil {
		s.log.ErrorContext(ctx, "falha ao enviar o link de redefinição forçada", "user_id", userID, "err", err)
	}
	return nil
}

type Domain struct {
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy *string   `json:"createdBy"`
}

func (s *Service) ListDomains(ctx context.Context) ([]Domain, error) {
	rows, err := s.q.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Domain, len(rows))
	for i, r := range rows {
		out[i] = Domain{Domain: r.Domain, CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy}
	}
	return out, nil
}

func normalizeDomain(d string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
}

func (s *Service) AddDomain(ctx context.Context, actor auth.Principal, domain string) error {
	domain = normalizeDomain(domain)
	if !domainPattern.MatchString(domain) {
		return ErrInvalidDomain
	}
	return s.audited(ctx, actor, "domain.add", "domain", domain, nil, func(q *dbgen.Queries, now time.Time) error {
		creator := actor.UserID
		return q.InsertDomain(ctx, dbgen.InsertDomainParams{Domain: domain, Now: now, CreatedBy: &creator})
	})
}

func (s *Service) RemoveDomain(ctx context.Context, actor auth.Principal, domain string) error {
	domain = normalizeDomain(domain)
	return s.audited(ctx, actor, "domain.remove", "domain", domain, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := q.LockDomains(ctx); err != nil {
			return err
		}
		if err := requireRow(q.DeleteDomain(ctx, domain)); err != nil {
			return err
		}
		remaining, err := q.CountDomains(ctx)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return ErrLastDomain // desfaz a remoção junto com a transação
		}
		return nil
	})
}

func (s *Service) AbortMatch(ctx context.Context, actor auth.Principal, matchID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "abortada pelo administrador"
	}
	err := s.audited(ctx, actor, "match.abort", "match", matchID, map[string]any{"reason": reason}, func(q *dbgen.Queries, now time.Time) error {
		return match.AbandonWith(ctx, q, matchID, reason, replay.EndAborted, now)
	})
	if err != nil {
		return err
	}
	if err := s.aborter.Abort(ctx, matchID); err != nil {
		s.log.ErrorContext(ctx, "falha ao encerrar a sala da partida abortada", "match_id", matchID, "err", err)
	}
	return nil
}

// LiveMatch é uma partida em andamento, sem placar ao vivo.
type LiveMatch struct {
	ID        string                `json:"id"`
	CreatedAt time.Time             `json:"createdAt"`
	Players   []match.PlayerSummary `json:"players"`
}

type MatchPage struct {
	Items      []LiveMatch `json:"items"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

// ListInProgressMatches lista as partidas em andamento, da mais antiga para a mais nova.
func (s *Service) ListInProgressMatches(ctx context.Context, cursor string) (MatchPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return MatchPage{}, err
	}
	rows, err := s.q.ListInProgressMatches(ctx, dbgen.ListInProgressMatchesParams{CursorCreatedAt: at, CursorID: id, Lim: pageSize + 1})
	if err != nil {
		return MatchPage{}, err
	}
	page := MatchPage{Items: []LiveMatch{}}
	ids := make([]string, 0, len(rows))
	for i, r := range rows {
		if i == pageSize {
			last := page.Items[pageSize-1]
			page.NextCursor = pagination.Encode(last.CreatedAt, last.ID)
			break
		}
		page.Items = append(page.Items, LiveMatch{ID: r.ID, CreatedAt: r.CreatedAt, Players: []match.PlayerSummary{}})
		ids = append(ids, r.ID)
	}
	if len(ids) == 0 {
		return page, nil
	}
	players, err := s.q.ListPlayersForMatches(ctx, ids)
	if err != nil {
		return MatchPage{}, err
	}
	byID := make(map[string]*LiveMatch, len(page.Items))
	for i := range page.Items {
		byID[page.Items[i].ID] = &page.Items[i]
	}
	for _, p := range players {
		if m := byID[p.MatchID]; m != nil {
			m.Players = append(m.Players, match.PlayerSummary{Seat: int(p.Seat), Username: p.Username, DisplayName: p.DisplayName})
		}
	}
	return page, nil
}

type UserSummary struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	DisplayName       string    `json:"displayName"`
	IsAdmin           bool      `json:"isAdmin"`
	EmailVerified     bool      `json:"emailVerified"`
	Disabled          bool      `json:"disabled"`
	MustResetPassword bool      `json:"mustResetPassword"`
	CreatedAt         time.Time `json:"createdAt"`
}

type UserPage struct {
	Items      []UserSummary `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *Service) ListUsers(ctx context.Context, query, cursor string) (UserPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return UserPage{}, err
	}
	rows, err := s.q.SearchUsers(ctx, dbgen.SearchUsersParams{Query: likeEscaper.Replace(strings.ToLower(strings.TrimSpace(query))),
		CursorCreatedAt: at, CursorID: id, Lim: pageSize + 1})
	if err != nil {
		return UserPage{}, err
	}
	page := UserPage{Items: []UserSummary{}}
	for i, u := range rows {
		if i == pageSize {
			last := page.Items[pageSize-1]
			page.NextCursor = pagination.Encode(last.CreatedAt, last.ID)
			break
		}
		page.Items = append(page.Items, UserSummary{ID: u.ID, Username: u.Username, Email: auth.Deref(u.Email), DisplayName: u.DisplayName,
			IsAdmin: u.IsAdmin, EmailVerified: u.EmailVerifiedAt != nil, Disabled: u.DisabledAt != nil,
			MustResetPassword: u.MustResetPassword, CreatedAt: u.CreatedAt})
	}
	return page, nil
}

type AuditEntry struct {
	ID            string          `json:"id"`
	ActorID       string          `json:"actorId"`
	ActorUsername string          `json:"actorUsername"`
	Action        string          `json:"action"`
	TargetType    string          `json:"targetType"`
	TargetID      string          `json:"targetId"`
	Details       json.RawMessage `json:"details"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type AuditPage struct {
	Items      []AuditEntry `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

func (s *Service) ListAudit(ctx context.Context, cursor string) (AuditPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return AuditPage{}, err
	}
	rows, err := s.q.ListAudit(ctx, dbgen.ListAuditParams{CursorCreatedAt: at, CursorID: id, Lim: pageSize + 1})
	if err != nil {
		return AuditPage{}, err
	}
	page := AuditPage{Items: []AuditEntry{}}
	for i, r := range rows {
		if i == pageSize {
			last := page.Items[pageSize-1]
			page.NextCursor = pagination.Encode(last.CreatedAt, last.ID)
			break
		}
		page.Items = append(page.Items, AuditEntry{ID: r.ID, ActorID: r.ActorID, ActorUsername: r.ActorUsername, Action: r.Action,
			TargetType: r.TargetType, TargetID: r.TargetID, Details: r.Details, CreatedAt: r.CreatedAt})
	}
	return page, nil
}

// lookupByLogin resolve e-mail ou username (normalizado) para o usuário.
func lookupByLogin(ctx context.Context, q *dbgen.Queries, login string) (dbgen.User, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	var user dbgen.User
	var err error
	if strings.Contains(login, "@") {
		user, err = q.GetHumanByEmail(ctx, &login)
	} else {
		user, err = q.GetHumanByUsername(ctx, login)
	}
	if err != nil {
		return dbgen.User{}, notFound(err)
	}
	return user, nil
}

// PromoteAdmin é usado pelo subcomando `server promote-admin <email|username>`; é o único caminho
// para criar um admin (nenhuma rota HTTP altera is_admin). A FK de audit_log.actor_id exige um
// usuário, então o ator da auditoria é o próprio alvo, marcado com details {"via":"cli"}.
func PromoteAdmin(ctx context.Context, pool *pgxpool.Pool, login string, now time.Time) error {
	q := dbgen.New(pool)
	user, err := lookupByLogin(ctx, q, login)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		qt := q.WithTx(tx)
		if _, err := qt.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: true, Now: now, ID: user.ID}); err != nil {
			return err
		}
		return writeAudit(ctx, qt, user.ID, "user.promote_admin", "user", user.ID, map[string]any{"via": "cli"}, now)
	})
}

// DemoteAdmin é usado por `server demote-admin <email|username>`. Recusa (ErrLastAdmin) rebaixar o
// último admin ativo. Auditoria: ator = o próprio alvo (FK de audit_log.actor_id), details {"via":"cli"}.
func DemoteAdmin(ctx context.Context, pool *pgxpool.Pool, login string, now time.Time) error {
	q := dbgen.New(pool)
	user, err := lookupByLogin(ctx, q, login)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		qt := q.WithTx(tx)
		if err := qt.LockAdmins(ctx); err != nil {
			return err
		}
		// Relê já com o lock: o estado pode ter mudado desde a leitura acima.
		target, err := qt.GetUserByID(ctx, user.ID)
		if err != nil {
			return notFound(err)
		}
		if !target.IsAdmin {
			return ErrNotAdmin
		}
		if target.DisabledAt == nil {
			count, err := qt.CountActiveAdmins(ctx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastAdmin
			}
		}
		if _, err := qt.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: false, Now: now, ID: user.ID}); err != nil {
			return err
		}
		return writeAudit(ctx, qt, user.ID, "user.demote_admin", "user", user.ID, map[string]any{"via": "cli"}, now)
	})
}
