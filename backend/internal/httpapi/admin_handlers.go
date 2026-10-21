package httpapi

import (
	"context"
	"net/http"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

func (s *Server) registerAdminRoutes() {
	s.handle("GET /api/admin/users", s.requireAdmin(s.handleAdminListUsers))
	s.handle("POST /api/admin/users/{id}/disable", s.requireAdmin(s.userAction(s.deps.Admin.DisableUser)))
	s.handle("POST /api/admin/users/{id}/enable", s.requireAdmin(s.userAction(s.deps.Admin.EnableUser)))
	s.handle("POST /api/admin/users/{id}/force-password-reset", s.requireAdmin(s.userAction(s.deps.Admin.ForcePasswordReset)))
	s.handle("GET /api/admin/email-domains", s.requireAdmin(s.handleAdminListDomains))
	s.handle("POST /api/admin/email-domains", s.requireAdmin(s.handleAdminAddDomain))
	s.handle("DELETE /api/admin/email-domains/{domain}", s.requireAdmin(s.handleAdminRemoveDomain))
	s.handle("GET /api/admin/matches", s.requireAdmin(s.handleAdminListMatches))
	s.handle("POST /api/admin/matches/{id}/abort", s.requireAdmin(s.handleAdminAbortMatch))
	s.handle("GET /api/admin/audit", s.requireAdmin(s.handleAdminAudit))
}

func noContent(s *Server, w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) userAction(action func(context.Context, auth.Principal, string) error) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		noContent(s, w, r, action(r.Context(), p, r.PathValue("id")))
	}
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	page, err := s.deps.Admin.ListUsers(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleAdminListDomains(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	domains, err := s.deps.Admin.ListDomains(r.Context())
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": domains})
}

type domainRequest struct {
	Domain string `json:"domain"`
}

func (s *Server) handleAdminAddDomain(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req domainRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	noContent(s, w, r, s.deps.Admin.AddDomain(r.Context(), p, req.Domain))
}

func (s *Server) handleAdminRemoveDomain(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	noContent(s, w, r, s.deps.Admin.RemoveDomain(r.Context(), p, r.PathValue("domain")))
}

type abortRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleAdminAbortMatch(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req abortRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	noContent(s, w, r, s.deps.Admin.AbortMatch(r.Context(), p, r.PathValue("id"), req.Reason))
}

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	page, err := s.deps.Admin.ListAudit(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// handleAdminListMatches lista as partidas em andamento; só o status in_progress existe.
func (s *Server) handleAdminListMatches(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	if st := r.URL.Query().Get("status"); st != "" && st != "in_progress" {
		s.writeError(w, r, &httpError{status: 400, code: "invalid_request", message: "Status inválido; use in_progress."})
		return
	}
	page, err := s.deps.Admin.ListInProgressMatches(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
