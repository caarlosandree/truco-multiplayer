package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

const (
	purposeVerify = "verify_email"
	purposeReset  = "password_reset"
	verifyTTL     = 24 * time.Hour
	resetTTL      = 30 * time.Minute
)

var errEmailTaken = errors.New("auth: e-mail já cadastrado")

type RegisterInput struct {
	Username, Email, DisplayName, Password string
}

type LoginResult struct {
	Token string
	User  User
}

func issueToken(ctx context.Context, q *dbgen.Queries, userID, purpose string, now time.Time, ttl time.Duration) (string, error) {
	if err := q.DeleteUnusedTokens(ctx, dbgen.DeleteUnusedTokensParams{UserID: userID, Purpose: purpose}); err != nil {
		return "", err
	}
	token, hash := NewToken()
	err := q.CreateUserToken(ctx, dbgen.CreateUserTokenParams{UserID: userID, Purpose: purpose, TokenHash: hash, Now: now, ExpiresAt: now.Add(ttl)})
	return token, err
}

func mapUserInsertError(err error) error {
	switch {
	case db.IsUniqueViolation(err, "users_username_key"):
		return ErrUsernameTaken
	case db.IsUniqueViolation(err, "users_email_key"):
		return errEmailTaken
	}
	return err
}

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	username, email := NormalizeUsername(in.Username), NormalizeEmail(in.Email)
	fields := FieldErrors{}
	if msg := ValidateUsername(username); msg != "" {
		fields["username"] = msg
	}
	domain, msg := ValidateEmail(email)
	if msg != "" {
		fields["email"] = msg
	}
	displayName, msg := ValidateDisplayName(in.DisplayName)
	if msg != "" {
		fields["displayName"] = msg
	}
	if msg := ValidatePassword(in.Password, username, email); msg != "" {
		fields["password"] = msg
	}
	if len(fields) > 0 {
		return fields
	}
	allowed, err := s.q.DomainAllowed(ctx, domain)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrEmailDomainNotAllowed
	}
	// Username é público: ocupado responde 409 antes de olhar o e-mail, senão a resposta
	// (202 ou 409) revelaria se o e-mail informado já tem conta.
	if _, err := s.q.GetUserByUsername(ctx, username); err == nil {
		return ErrUsernameTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// O hash vem antes da checagem de e-mail existente para os dois caminhos levarem o mesmo tempo.
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return err
	}
	if existing, err := s.q.GetUserByEmail(ctx, &email); err == nil {
		s.sendLater(mail.AccountExists(Deref(existing.Email), existing.DisplayName, s.link("/entrar"), s.link("/esqueci-senha")))
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	now := s.now()
	var token string
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Username: username, Email: &email, DisplayName: displayName, PasswordHash: &hash, Now: now})
		if err != nil {
			return mapUserInsertError(err)
		}
		token, err = issueToken(ctx, q, user.ID, purposeVerify, now, verifyTTL)
		return err
	})
	if errors.Is(err, errEmailTaken) {
		return nil // corrida com outro cadastro do mesmo e-mail: mesma resposta de sucesso
	}
	if err != nil {
		return err
	}
	s.sendLater(mail.VerifyEmail(email, displayName, s.link("/verificar-email?token="+token)))
	return nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return ErrInvalidToken
	}
	now := s.now()
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		userID, err := q.ConsumeUserToken(ctx, dbgen.ConsumeUserTokenParams{Now: now, TokenHash: HashToken(token), Purpose: purposeVerify})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		return q.MarkEmailVerified(ctx, dbgen.MarkEmailVerifiedParams{Now: now, ID: userID})
	})
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, ptr(NormalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil || user.DisabledAt != nil {
		return nil
	}
	return s.sendVerification(ctx, user)
}

func (s *Service) sendVerification(ctx context.Context, user dbgen.User) error {
	token, err := issueToken(ctx, s.q, user.ID, purposeVerify, s.now(), verifyTTL)
	if err != nil {
		return err
	}
	s.sendLater(mail.VerifyEmail(Deref(user.Email), user.DisplayName, s.link("/verificar-email?token="+token)))
	return nil
}

func (s *Service) Login(ctx context.Context, login, password string, meta SessionMeta) (LoginResult, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	var row dbgen.User
	var err error
	if strings.Contains(login, "@") {
		row, err = s.q.GetHumanByEmail(ctx, &login)
	} else {
		row, err = s.q.GetHumanByUsername(ctx, login)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		s.hasher.VerifyDummy(password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if row.PasswordHash == nil { // conta de bot: não tem credenciais
		s.hasher.VerifyDummy(password)
		return LoginResult{}, ErrInvalidCredentials
	}
	ok, needsRehash, err := s.hasher.Verify(password, *row.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	switch {
	case row.DisabledAt != nil:
		return LoginResult{}, ErrAccountDisabled
	case row.EmailVerifiedAt == nil:
		return LoginResult{}, ErrEmailNotVerified
	case row.MustResetPassword:
		return LoginResult{}, ErrPasswordResetRequired
	}
	if needsRehash {
		s.rehash(ctx, row.ID, password)
	}
	token, err := s.createSession(ctx, s.q, row.ID, meta)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: userFromRow(row)}, nil
}

// rehash é melhor esforço: falhar aqui não impede o login.
func (s *Service) rehash(ctx context.Context, userID, password string) {
	hash, err := s.hasher.Hash(password)
	if err == nil {
		err = s.q.SetPasswordHash(ctx, dbgen.SetPasswordHashParams{PasswordHash: &hash, Now: s.now(), ID: userID})
	}
	if err != nil {
		s.log.Error("falha ao atualizar hash de senha", "user_id", userID, "err", err)
	}
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, ptr(NormalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	switch {
	case user.DisabledAt != nil:
		return nil
	case user.EmailVerifiedAt == nil:
		return s.sendVerification(ctx, user)
	}
	return s.SendPasswordReset(ctx, user.ID, false)
}

// SendPasswordReset emite um link de reset; forced=true usa o texto do reset pedido pelo admin.
func (s *Service) SendPasswordReset(ctx context.Context, userID string, forced bool) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Kind == "bot") { // bot não tem e-mail nem senha
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	token, err := issueToken(ctx, s.q, user.ID, purposeReset, s.now(), resetTTL)
	if err != nil {
		return err
	}
	link := s.link("/redefinir-senha?token=" + token)
	if forced {
		s.sendLater(mail.ForcedPasswordReset(Deref(user.Email), user.DisplayName, link))
	} else {
		s.sendLater(mail.PasswordReset(Deref(user.Email), user.DisplayName, link))
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if token == "" {
		return ErrInvalidToken
	}
	now, hash := s.now(), HashToken(token)
	userID, err := s.q.GetValidUserToken(ctx, dbgen.GetValidUserTokenParams{TokenHash: hash, Purpose: purposeReset, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	// Valida antes de consumir: um erro de digitação não queima o link.
	if msg := ValidatePassword(password, user.Username, Deref(user.Email)); msg != "" {
		return FieldErrors{"password": msg}
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.ConsumeUserToken(ctx, dbgen.ConsumeUserTokenParams{Now: now, TokenHash: hash, Purpose: purposeReset}); errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		} else if err != nil {
			return err
		}
		if err := q.CompletePasswordReset(ctx, dbgen.CompletePasswordResetParams{PasswordHash: &passwordHash, Now: now, ID: user.ID}); err != nil {
			return err
		}
		if err := q.DeleteUnusedTokens(ctx, dbgen.DeleteUnusedTokensParams{UserID: user.ID, Purpose: purposeReset}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, user.ID)
	})
	if err != nil {
		return err
	}
	s.sendLater(mail.PasswordChanged(Deref(user.Email), user.DisplayName))
	return nil
}

// ChangePassword troca a senha, derruba todas as sessões e devolve o token de uma sessão nova.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string, meta SessionMeta) (string, error) {
	user, err := s.q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return "", err
	}
	ok, _, err := s.hasher.Verify(current, Deref(user.PasswordHash))
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	if msg := ValidatePassword(next, user.Username, Deref(user.Email)); msg != "" {
		return "", FieldErrors{"newPassword": msg}
	}
	passwordHash, err := s.hasher.Hash(next)
	if err != nil {
		return "", err
	}
	var token string
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.SetPasswordHash(ctx, dbgen.SetPasswordHashParams{PasswordHash: &passwordHash, Now: s.now(), ID: user.ID}); err != nil {
			return err
		}
		// Links de reset emitidos antes da troca não podem desfazê-la.
		if err := q.DeleteUnusedTokens(ctx, dbgen.DeleteUnusedTokensParams{UserID: user.ID, Purpose: purposeReset}); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
			return err
		}
		token, err = s.createSession(ctx, q, user.ID, meta)
		return err
	})
	if err != nil {
		return "", err
	}
	s.sendLater(mail.PasswordChanged(Deref(user.Email), user.DisplayName))
	return token, nil
}

func (s *Service) UpdateDisplayName(ctx context.Context, userID, name string) (User, error) {
	name, msg := ValidateDisplayName(name)
	if msg != "" {
		return User{}, FieldErrors{"displayName": msg}
	}
	row, err := s.q.UpdateDisplayName(ctx, dbgen.UpdateDisplayNameParams{DisplayName: name, Now: s.now(), ID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

func ptr(s string) *string { return &s }

// Deref lê um campo anulável de credencial (e-mail, hash). Nulo só ocorre em contas de bot, que nunca chegam aos fluxos de conta.
func Deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
