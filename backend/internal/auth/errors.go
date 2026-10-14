package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("auth: usuário ou senha incorretos")
	ErrEmailNotVerified      = errors.New("auth: e-mail não confirmado")
	ErrAccountDisabled       = errors.New("auth: conta desativada")
	ErrPasswordResetRequired = errors.New("auth: redefinição de senha obrigatória")
	ErrUsernameTaken         = errors.New("auth: nome de usuário em uso")
	ErrEmailDomainNotAllowed = errors.New("auth: domínio de e-mail não permitido")
	ErrInvalidToken          = errors.New("auth: link inválido ou expirado")
	ErrUnauthenticated       = errors.New("auth: não autenticado")
	ErrUserNotFound          = errors.New("auth: usuário não encontrado")
)
