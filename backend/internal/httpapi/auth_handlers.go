package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

// Mensagens idênticas em todos os casos para não revelar se o e-mail existe.
var (
	registerAccepted = map[string]string{"message": "Se os dados estiverem corretos, você receberá um e-mail para confirmar a conta."}
	emailAccepted    = map[string]string{"message": "Se houver uma conta com este e-mail, enviaremos as instruções."}
)

func (s *Server) registerAuthRoutes() {
	s.mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/auth/verify-email", s.handleVerifyEmail)
	s.mux.HandleFunc("POST /api/auth/resend-verification", s.handleResendVerification)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.Handle("POST /api/auth/logout-all", s.requireAuth(s.handleLogoutAll))
	s.mux.HandleFunc("POST /api/auth/password/forgot", s.handleForgotPassword)
	s.mux.HandleFunc("POST /api/auth/password/reset", s.handleResetPassword)
	s.mux.Handle("GET /api/me", s.requireAuth(s.handleMe))
	s.mux.Handle("PATCH /api/me", s.requireAuth(s.handleUpdateMe))
	s.mux.Handle("POST /api/me/password", s.requireAuth(s.handleChangePassword))
}

type registerRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.registerIP, s.clientIP(r)) {
		return
	}
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	err := s.deps.Auth.Register(r.Context(), auth.RegisterInput{Username: req.Username, Email: req.Email, DisplayName: req.DisplayName, Password: req.Password})
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, registerAccepted)
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.tokenIP, s.clientIP(r)) {
		return
	}
	var req tokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.deps.Auth.VerifyEmail(r.Context(), req.Token); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type emailRequest struct {
	Email string `json:"email"`
}

// emailAction aplica os limites por IP e por e-mail e responde sempre 202.
func (s *Server) emailAction(w http.ResponseWriter, r *http.Request, action func(email string) error) {
	if !s.allow(w, r, s.recoveryIP, s.clientIP(r)) {
		return
	}
	var req emailRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if !s.allow(w, r, s.recoveryEmail, auth.NormalizeEmail(req.Email)) {
		return
	}
	if err := action(req.Email); err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, emailAccepted)
}

func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	s.emailAction(w, r, func(email string) error { return s.deps.Auth.ResendVerification(r.Context(), email) })
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	s.emailAction(w, r, func(email string) error { return s.deps.Auth.ForgotPassword(r.Context(), email) })
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.loginIP, s.clientIP(r)) {
		return
	}
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	key := strings.ToLower(strings.TrimSpace(req.Login))
	if locked, retry := s.loginLockout.Locked(key); locked {
		s.writeTooMany(w, r, retry)
		return
	}
	res, err := s.deps.Auth.Login(r.Context(), req.Login, req.Password, s.sessionMeta(r))
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.loginLockout.Fail(key)
	}
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.loginLockout.Reset(key)
	s.deps.Auth.Cookies().Set(w, res.Token)
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": res.User})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookies := s.deps.Auth.Cookies()
	if err := s.deps.Auth.Logout(r.Context(), cookies.Read(r)); err != nil {
		s.writeError(w, r, err)
		return
	}
	cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if err := s.deps.Auth.LogoutAll(r.Context(), p.UserID); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.deps.Auth.Cookies().Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.allow(w, r, s.tokenIP, s.clientIP(r)) {
		return
	}
	var req resetRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.deps.Auth.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	user, err := s.deps.Auth.GetUser(r.Context(), p.UserID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": user})
}

type updateMeRequest struct {
	DisplayName string `json:"displayName"`
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req updateMeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	user, err := s.deps.Auth.UpdateDisplayName(r.Context(), p.UserID, req.DisplayName)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]auth.User{"user": user})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req changePasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	token, err := s.deps.Auth.ChangePassword(r.Context(), p, req.CurrentPassword, req.NewPassword, s.sessionMeta(r))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.deps.Auth.Cookies().Set(w, token)
	w.WriteHeader(http.StatusNoContent)
}
