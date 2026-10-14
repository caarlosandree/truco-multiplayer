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
)

// MatchAborter encerra a sala viva de uma partida abortada. A Spec 3 implementa.
type MatchAborter interface {
	Abort(ctx context.Context, matchID string) error
}

type NoopAborter struct{}

func (NoopAborter) Abort(context.Context, string) error { return nil }

var (
	ErrNotFound      = errors.New("admin: não encontrado")
	ErrSelfAction    = errors.New("admin: não é possível aplicar esta ação a si mesmo")
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
	Now     func() time.Time
	Logger  *slog.Logger
}

type Service struct {
	pool    *pgxpool.Pool
	q       *dbgen.Queries
	auth    *auth.Service
	aborter MatchAborter
	now     func() time.Time
	log     *slog.Logger
}

func NewService(o Options) *Service {
	s := &Service{pool: o.Pool, q: dbgen.New(o.Pool), auth: o.Auth, aborter: o.Aborter, now: o.Now, log: o.Logger}
	if s.aborter == nil {
		s.aborter = NoopAborter{}
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

func (s *Service) DisableUser(ctx context.Context, actor auth.Principal, userID string) error {
	if userID == actor.UserID {
		return ErrSelfAction
	}
	return s.audited(ctx, actor, "user.disable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: &now, Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
}

func (s *Service) EnableUser(ctx context.Context, actor auth.Principal, userID string) error {
	return s.audited(ctx, actor, "user.enable", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		return requireRow(q.SetUserDisabled(ctx, dbgen.SetUserDisabledParams{DisabledAt: nil, Now: now, ID: userID}))
	})
}

func (s *Service) ForcePasswordReset(ctx context.Context, actor auth.Principal, userID string) error {
	err := s.audited(ctx, actor, "user.force_password_reset", "user", userID, nil, func(q *dbgen.Queries, now time.Time) error {
		if err := requireRow(q.SetMustResetPassword(ctx, dbgen.SetMustResetPasswordParams{Now: now, ID: userID})); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
	if err != nil {
		return err
	}
	return s.auth.SendPasswordReset(ctx, userID, true)
}

func (s *Service) SetAdmin(ctx context.Context, actor auth.Principal, userID string, isAdmin bool) error {
	details := map[string]any{"isAdmin": isAdmin}
	return s.audited(ctx, actor, "user.set_admin", "user", userID, details, func(q *dbgen.Queries, now time.Time) error {
		if err := q.LockAdmins(ctx); err != nil {
			return err
		}
		if !isAdmin {
			target, err := q.GetUserByID(ctx, userID)
			if err != nil {
				return notFound(err)
			}
			if target.IsAdmin && target.DisabledAt == nil {
				count, err := q.CountActiveAdmins(ctx)
				if err != nil {
					return err
				}
				if count <= 1 {
					return ErrLastAdmin
				}
			}
		}
		return requireRow(q.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: isAdmin, Now: now, ID: userID}))
	})
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
		return match.AbandonWith(ctx, q, matchID, reason, now)
	})
	if err != nil {
		return err
	}
	if err := s.aborter.Abort(ctx, matchID); err != nil {
		s.log.ErrorContext(ctx, "falha ao encerrar a sala da partida abortada", "match_id", matchID, "err", err)
	}
	return nil
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
		page.Items = append(page.Items, UserSummary{ID: u.ID, Username: u.Username, Email: u.Email, DisplayName: u.DisplayName,
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

// PromoteAdmin é usado pelo subcomando `server promote-admin <email|username>` para criar o primeiro admin.
func PromoteAdmin(ctx context.Context, pool *pgxpool.Pool, login string, now time.Time) error {
	q := dbgen.New(pool)
	login = strings.ToLower(strings.TrimSpace(login))
	var user dbgen.User
	var err error
	if strings.Contains(login, "@") {
		user, err = q.GetUserByEmail(ctx, login)
	} else {
		user, err = q.GetUserByUsername(ctx, login)
	}
	if err != nil {
		return notFound(err)
	}
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		qt := q.WithTx(tx)
		if _, err := qt.SetUserAdmin(ctx, dbgen.SetUserAdminParams{IsAdmin: true, Now: now, ID: user.ID}); err != nil {
			return err
		}
		return writeAudit(ctx, qt, user.ID, "user.promote_admin", "user", user.ID, map[string]any{"via": "cli"}, now)
	})
}
