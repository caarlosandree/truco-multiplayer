package httpapi

import (
	"context"
	"net/http"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
)

func (s *Server) registerAdminRoutes() {
	s.mux.Handle("GET /api/admin/users", s.requireAdmin(s.handleAdminListUsers))
	s.mux.Handle("POST /api/admin/users/{id}/disable", s.requireAdmin(s.userAction(s.deps.Admin.DisableUser)))
	s.mux.Handle("POST /api/admin/users/{id}/enable", s.requireAdmin(s.userAction(s.deps.Admin.EnableUser)))
	s.mux.Handle("POST /api/admin/users/{id}/force-password-reset", s.requireAdmin(s.userAction(s.deps.Admin.ForcePasswordReset)))
	s.mux.Handle("PUT /api/admin/users/{id}/admin", s.requireAdmin(s.handleAdminSetAdmin))
	s.mux.Handle("GET /api/admin/email-domains", s.requireAdmin(s.handleAdminListDomains))
	s.mux.Handle("POST /api/admin/email-domains", s.requireAdmin(s.handleAdminAddDomain))
	s.mux.Handle("DELETE /api/admin/email-domains/{domain}", s.requireAdmin(s.handleAdminRemoveDomain))
	s.mux.Handle("POST /api/admin/matches/{id}/abort", s.requireAdmin(s.handleAdminAbortMatch))
	s.mux.Handle("GET /api/admin/audit", s.requireAdmin(s.handleAdminAudit))
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

type setAdminRequest struct {
	IsAdmin *bool `json:"isAdmin"`
}

func (s *Server) handleAdminSetAdmin(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var req setAdminRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	if req.IsAdmin == nil {
		s.writeError(w, r, errInvalidJSON)
		return
	}
	noContent(s, w, r, s.deps.Admin.SetAdmin(r.Context(), p, r.PathValue("id"), *req.IsAdmin))
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
