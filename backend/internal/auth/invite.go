package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
)

// ActiveUserByUsername devolve o usuário que pode receber convite: existente, verificado e não desativado.
// Qualquer outro caso é ErrUserNotFound, sem distinguir o motivo.
func (s *Service) ActiveUserByUsername(ctx context.Context, username string) (User, error) {
	if !pagination.ValidText(username) { // NUL ou UTF-8 inválido o Postgres recusa: nunca chega lá
		return User{}, ErrUserNotFound
	}
	row, err := s.q.GetHumanByUsername(ctx, NormalizeUsername(username))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	if row.DisabledAt != nil || row.EmailVerifiedAt == nil {
		return User{}, ErrUserNotFound
	}
	return userFromRow(row), nil
}

// SendRoomInvite manda o e-mail do convite com o link do token. Nunca bloqueia: a busca do endereço e o envio
// rodam fora da chamada, e o desligamento gracioso espera por eles (Wait). O endereço é lido aqui, no servidor;
// nunca passa pelo lobby nem pelo cliente.
func (s *Service) SendRoomInvite(userID, inviterName, roomName, token string) {
	s.pending.Add(1)
	s.background(func() {
		defer s.pending.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		user, err := s.GetUser(ctx, userID)
		if err != nil {
			s.log.Error("convite por e-mail: destinatário não encontrado", "user_id", userID, "err", err)
			return
		}
		s.sendLater(mail.RoomInvite(user.Email, inviterName, roomName, s.link("/entrar-convite/"+token)))
	})
}
