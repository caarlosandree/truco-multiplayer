package lobby

import (
	"context"
	"errors"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

// Invitee é a pessoa que pode receber um convite. Não leva e-mail: o endereço fica do lado do servidor de contas.
type Invitee struct {
	ID, Username, DisplayName string
}

// ErrNoInvitee indica que o username não existe, está desativado ou não confirmou o e-mail.
var ErrNoInvitee = errors.New("lobby: convidado inexistente ou inativo")

// People é o que o lobby precisa do cadastro de contas para convidar.
type People interface {
	// Invitee resolve o username; ErrNoInvitee quando a pessoa não pode ser convidada.
	Invitee(ctx context.Context, username string) (Invitee, error)
	// MailInvite manda o e-mail do convite a quem estava offline. Nunca bloqueia: o envio é assíncrono.
	MailInvite(inviteeID, inviterName, roomName, token string)
}

// nullPeople é usado quando o lobby sobe sem cadastro (testes de transporte): ninguém pode ser convidado.
type nullPeople struct{}

func (nullPeople) Invitee(context.Context, string) (Invitee, error) { return Invitee{}, ErrNoInvitee }
func (nullPeople) MailInvite(string, string, string, string)        {}

type authPeople struct{ a *auth.Service }

// NewPeople liga o lobby ao serviço de contas.
func NewPeople(a *auth.Service) People { return authPeople{a} }

func (p authPeople) Invitee(ctx context.Context, username string) (Invitee, error) {
	u, err := p.a.ActiveUserByUsername(ctx, username)
	if errors.Is(err, auth.ErrUserNotFound) {
		return Invitee{}, ErrNoInvitee
	}
	if err != nil {
		return Invitee{}, err
	}
	return Invitee{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName}, nil
}

func (p authPeople) MailInvite(inviteeID, inviterName, roomName, token string) {
	p.a.SendRoomInvite(inviteeID, inviterName, roomName, token)
}
