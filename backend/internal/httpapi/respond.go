package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

const maxBodyBytes = 16 << 10

type httpError struct {
	status  int
	code    string
	message string
	fields  map[string]string
}

func (e *httpError) Error() string { return e.code + ": " + e.message }

var (
	errInvalidJSON      = &httpError{status: 400, code: "invalid_request", message: "Corpo da requisição inválido."}
	errUnsupportedMedia = &httpError{status: 415, code: "invalid_request", message: "Envie o corpo como application/json."}
	errBodyTooLarge     = &httpError{status: 413, code: "invalid_request", message: "Corpo da requisição grande demais."}
	errForbiddenOrigin  = &httpError{status: 403, code: "forbidden_origin", message: "Origem da requisição não permitida."}
	errForbidden        = &httpError{status: 403, code: "forbidden", message: "Acesso restrito a administradores."}
	errNotFoundRoute    = &httpError{status: 404, code: "not_found", message: "Recurso não encontrado."}
	errTooMany          = &httpError{status: 429, code: "too_many_attempts", message: "Muitas tentativas. Aguarde um pouco e tente de novo."}
	errMatchInProgress  = &httpError{status: 409, code: "match_in_progress", message: "A partida ainda está em andamento."}
	errInternal         = &httpError{status: 500, code: "internal", message: "Erro interno. Tente novamente."}
)

func conflict(message string) *httpError {
	return &httpError{status: 409, code: "conflict", message: message}
}

func invalidField(field, reason string) *httpError {
	return &httpError{status: 400, code: "validation_failed", message: "Verifique os campos destacados.", fields: map[string]string{field: reason}}
}

// classify traduz erros de domínio para status e código da API.
func classify(err error) *httpError {
	var he *httpError
	if errors.As(err, &he) {
		return he
	}
	var fields auth.FieldErrors
	if errors.As(err, &fields) {
		return &httpError{status: 400, code: "validation_failed", message: "Verifique os campos destacados.", fields: fields}
	}
	var corrupt *replay.CorruptLogError
	switch {
	case errors.Is(err, auth.ErrEmailDomainNotAllowed):
		return &httpError{status: 403, code: "email_domain_not_allowed", message: "O cadastro só é permitido com o e-mail da empresa."}
	case errors.Is(err, auth.ErrUsernameTaken):
		return &httpError{status: 409, code: "username_taken", message: "Este nome de usuário já está em uso."}
	case errors.Is(err, auth.ErrInvalidToken):
		return &httpError{status: 400, code: "invalid_token", message: "Link inválido ou expirado."}
	case errors.Is(err, auth.ErrInvalidCredentials):
		return &httpError{status: 401, code: "invalid_credentials", message: "Usuário ou senha incorretos."}
	case errors.Is(err, auth.ErrEmailNotVerified):
		return &httpError{status: 403, code: "email_not_verified", message: "Confirme seu e-mail antes de entrar."}
	case errors.Is(err, auth.ErrAccountDisabled):
		return &httpError{status: 403, code: "account_disabled", message: "Esta conta está desativada."}
	case errors.Is(err, auth.ErrPasswordResetRequired):
		return &httpError{status: 403, code: "password_reset_required", message: "Você precisa redefinir sua senha. Use \"Esqueci minha senha\"."}
	case errors.Is(err, auth.ErrUnauthenticated):
		return &httpError{status: 401, code: "unauthenticated", message: "Faça login para continuar."}
	case errors.Is(err, auth.ErrUserNotFound), errors.Is(err, stats.ErrUserNotFound),
		errors.Is(err, match.ErrNotFound), errors.Is(err, admin.ErrNotFound):
		return errNotFoundRoute
	case errors.Is(err, pagination.ErrInvalidCursor):
		return invalidField("cursor", "cursor inválido")
	case errors.Is(err, stats.ErrInvalidRankingQuery):
		return &httpError{status: 400, code: "invalid_request", message: "Período ou métrica do ranking inválidos."}
	case errors.Is(err, stats.ErrInvalidQuery):
		return invalidField("q", "busca inválida")
	case errors.Is(err, admin.ErrInvalidDomain):
		return invalidField("domain", "domínio inválido")
	case errors.Is(err, admin.ErrSelfAction):
		return conflict("Você não pode aplicar esta ação à sua própria conta.")
	case errors.Is(err, admin.ErrLastAdmin):
		return conflict("É preciso manter ao menos um administrador ativo.")
	case errors.Is(err, admin.ErrLastDomain):
		return conflict("É preciso manter ao menos um domínio permitido.")
	case errors.Is(err, match.ErrMatchClosed):
		return conflict("A partida não está em andamento.")
	case errors.Is(err, replay.ErrIncompatibleVersion), errors.As(err, &corrupt):
		return conflict("O replay desta partida não está disponível.")
	}
	return errInternal
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	he := classify(err)
	if he.status >= 500 {
		s.log.ErrorContext(r.Context(), "erro interno", "request_id", requestIDFrom(r.Context()),
			"method", r.Method, "path", r.URL.Path, "err", err)
	}
	writeJSON(w, he.status, errorBody{Error: errorPayload{Code: he.code, Message: he.message, Fields: he.fields}})
}

// decodeJSON exige application/json, limita o corpo a 16 KB, rejeita campos
// desconhecidos e qualquer coisa depois do objeto.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		return errUnsupportedMedia
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errBodyTooLarge
		}
		return errInvalidJSON
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errInvalidJSON
	}
	return nil
}
