package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func (s *Server) registerReadRoutes() {
	s.mux.Handle("GET /api/ranking", s.requireAuth(s.handleRanking))
	s.mux.Handle("GET /api/users/{username}", s.requireAuth(s.handleProfile))
	s.mux.Handle("GET /api/matches", s.requireAuth(s.handleHistory))
	s.mux.Handle("GET /api/matches/{id}", s.requireAuth(s.handleMatch))
	s.mux.Handle("GET /api/matches/{id}/replay", s.requireAuth(s.handleReplay))
}

func (s *Server) handleRanking(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := s.deps.Stats.Ranking(r.Context(), limit)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	profile, err := s.deps.Stats.Profile(r.Context(), r.PathValue("username"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	username := r.URL.Query().Get("user")
	if username == "" {
		username = p.Username
	}
	page, err := s.deps.Stats.History(r.Context(), username, r.URL.Query().Get("cursor"), 0)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleMatch(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	summary, err := s.deps.Matches.Summary(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if summary.Players == nil {
		summary.Players = []match.PlayerSummary{}
	}
	writeJSON(w, http.StatusOK, summary)
}

type replayPlayer struct {
	Seat        int    `json:"seat"`
	DisplayName string `json:"displayName"`
}

type replayResponse struct {
	ID      string         `json:"id"`
	Players []replayPlayer `json:"players"`
	Steps   []replay.Frame `json:"steps"`
}

// handleReplay só atende partidas encerradas: durante o jogo, o replay revelaria cartas ocultas.
func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request, _ auth.Principal) {
	rec, err := s.deps.Matches.Load(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if rec.Status == replay.StatusInProgress {
		s.writeError(w, r, errMatchInProgress)
		return
	}
	frames, err := replay.BuildFrames(rec)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	for i := range frames {
		if frames[i].Events == nil {
			frames[i].Events = []engine.Event{}
		}
	}
	body := replayResponse{ID: rec.ID, Players: make([]replayPlayer, 0, len(rec.Players)), Steps: frames}
	for seat, p := range rec.Players {
		body.Players = append(body.Players, replayPlayer{Seat: seat, DisplayName: p.DisplayName})
	}
	writeJSONMaybeGzip(w, r, http.StatusOK, body)
}

func writeJSONMaybeGzip(w http.ResponseWriter, r *http.Request, status int, v any) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		writeJSON(w, status, v)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.WriteHeader(status)
	gz := gzip.NewWriter(w)
	defer gz.Close()
	_ = json.NewEncoder(gz).Encode(v)
}
