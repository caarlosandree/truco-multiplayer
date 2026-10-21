package stats

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var ErrUserNotFound = errors.New("stats: usuário não encontrado")

// ErrInvalidQuery indica busca com UTF-8 inválido, byte NUL ou tamanho excessivo.
var ErrInvalidQuery = errors.New("stats: busca inválida")

const maxQueryRunes = 64

// Reader serve as leituras públicas: ranking, perfil e histórico de partidas.
type Reader struct {
	q          *dbgen.Queries
	minMatches int
}

func NewReader(pool *pgxpool.Pool, minMatches int) *Reader {
	return &Reader{q: dbgen.New(pool), minMatches: minMatches}
}

type RankingEntry struct {
	Position      int     `json:"position"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"displayName"`
	MatchesPlayed int     `json:"matchesPlayed"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	WinRate       float64 `json:"winRate"`
}

type Totals struct {
	MatchesPlayed   int     `json:"matchesPlayed"`
	Wins            int     `json:"wins"`
	Losses          int     `json:"losses"`
	WinRate         float64 `json:"winRate"`
	PointsFor       int     `json:"pointsFor"`
	PointsAgainst   int     `json:"pointsAgainst"`
	HandsPlayed     int     `json:"handsPlayed"`
	HandsWon        int     `json:"handsWon"`
	RaisesCalled    int     `json:"raisesCalled"`
	RaisesFolded    int     `json:"raisesFolded"`
	RaisesWonByFold int     `json:"raisesWonByFold"`
	ElevenHands     int     `json:"elevenHands"`
	ElevenRuns      int     `json:"elevenRuns"`
	ElevenWon       int     `json:"elevenWon"`
	IronHands       int     `json:"ironHands"`
	IronWon         int     `json:"ironWon"`
	Punished        int     `json:"punished"`
	Timeouts        int     `json:"timeouts"`
}

// Partner é um parceiro de dupla frequente.
type Partner struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Matches     int     `json:"matches"`
	Wins        int     `json:"wins"`
	WinRate     float64 `json:"winRate"`
}

type Profile struct {
	Username      string        `json:"username"`
	DisplayName   string        `json:"displayName"`
	CreatedAt     time.Time     `json:"createdAt"`
	Stats         Totals        `json:"stats"`
	RecentMatches []HistoryItem `json:"recentMatches"`
	Partners      []Partner     `json:"partners"`
}

// MatchPlayer é um jogador sentado numa partida do histórico.
type MatchPlayer struct {
	Seat        int    `json:"seat"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// DirectoryEntry é o que o diretório de pessoas expõe: nunca o e-mail.
type DirectoryEntry struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type DirectoryPage struct {
	Items      []DirectoryEntry `json:"items"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

const (
	recentMatchesLimit = 5
	partnersLimit      = 5
	directoryPageSize  = 20
)

// likeEscaper neutraliza os curingas do LIKE na busca digitada pelo usuário.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

type HistoryItem struct {
	ID         string           `json:"id"`
	Status     replay.Status    `json:"status"`
	Team       int              `json:"team"`
	Won        bool             `json:"won"`
	WinnerTeam *int             `json:"winnerTeam"`
	Score      *[2]int          `json:"score"`
	CreatedAt  time.Time        `json:"createdAt"`
	FinishedAt *time.Time       `json:"finishedAt"`
	Players    []MatchPlayer    `json:"players"`
	EndReason  replay.EndReason `json:"endReason"`
}

type HistoryPage struct {
	Items      []HistoryItem `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

func clamp(v, fallback, maximum int) int {
	if v <= 0 {
		return fallback
	}
	return min(v, maximum)
}

func winRate(wins, played int) float64 {
	if played == 0 {
		return 0
	}
	return float64(wins) / float64(played)
}

// Ranking devolve os melhores jogadores (limit <= 0 → 50; máximo 100).
func (r *Reader) Ranking(ctx context.Context, limit int) ([]RankingEntry, error) {
	rows, err := r.q.Ranking(ctx, dbgen.RankingParams{MinMatches: int32(r.minMatches), Lim: int32(clamp(limit, 50, 100))})
	if err != nil {
		return nil, err
	}
	out := make([]RankingEntry, len(rows))
	for i, row := range rows {
		out[i] = RankingEntry{Position: i + 1, Username: row.Username, DisplayName: row.DisplayName,
			MatchesPlayed: int(row.MatchesPlayed), Wins: int(row.Wins), Losses: int(row.Losses),
			WinRate: winRate(int(row.Wins), int(row.MatchesPlayed))}
	}
	return out, nil
}

// user resolve o username; conta desativada só existe para quem é admin.
func (r *Reader) user(ctx context.Context, username string, viewerIsAdmin bool) (dbgen.User, error) {
	u, err := r.q.GetUserByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.User{}, ErrUserNotFound
	}
	if err != nil {
		return dbgen.User{}, err
	}
	if u.DisabledAt != nil && !viewerIsAdmin {
		return dbgen.User{}, ErrUserNotFound
	}
	return u, nil
}

// Profile devolve o perfil público; jogador sem partidas tem estatísticas zeradas.
func (r *Reader) Profile(ctx context.Context, username string, viewerIsAdmin bool) (Profile, error) {
	u, err := r.user(ctx, username, viewerIsAdmin)
	if err != nil {
		return Profile{}, err
	}
	out := Profile{Username: u.Username, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt}
	recent, err := r.history(ctx, u.ID, nil, nil, recentMatchesLimit)
	if err != nil {
		return Profile{}, err
	}
	out.RecentMatches = recent.Items
	partners, err := r.q.ListPartners(ctx, dbgen.ListPartnersParams{UserID: u.ID, Lim: partnersLimit})
	if err != nil {
		return Profile{}, err
	}
	out.Partners = make([]Partner, len(partners))
	for i, row := range partners {
		out.Partners[i] = Partner{Username: row.Username, DisplayName: row.DisplayName,
			Matches: int(row.Matches), Wins: int(row.Wins), WinRate: winRate(int(row.Wins), int(row.Matches))}
	}
	p, err := r.q.GetPlayerStats(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Profile{}, err
	}
	out.Stats = Totals{
		MatchesPlayed: int(p.MatchesPlayed), Wins: int(p.Wins), Losses: int(p.Losses),
		WinRate:   winRate(int(p.Wins), int(p.MatchesPlayed)),
		PointsFor: int(p.PointsFor), PointsAgainst: int(p.PointsAgainst),
		HandsPlayed: int(p.HandsPlayed), HandsWon: int(p.HandsWon),
		RaisesCalled: int(p.RaisesCalled), RaisesFolded: int(p.RaisesFolded), RaisesWonByFold: int(p.RaisesWonByFold),
		ElevenHands: int(p.ElevenHands), ElevenRuns: int(p.ElevenRuns), ElevenWon: int(p.ElevenWon),
		IronHands: int(p.IronHands), IronWon: int(p.IronWon), Punished: int(p.Punished), Timeouts: int(p.Timeouts),
	}
	return out, nil
}

// History devolve partidas encerradas do jogador, da mais recente para a mais antiga
// (limit <= 0 → 20; máximo 50). Cursor inválido devolve pagination.ErrInvalidCursor.
// Conta desativada só é visível para admin.
func (r *Reader) History(ctx context.Context, username, cursor string, limit int, viewerIsAdmin bool) (HistoryPage, error) {
	at, id, err := pagination.Decode(cursor)
	if err != nil {
		return HistoryPage{}, err
	}
	u, err := r.user(ctx, username, viewerIsAdmin)
	if err != nil {
		return HistoryPage{}, err
	}
	return r.history(ctx, u.ID, at, id, clamp(limit, 20, 50))
}

func (r *Reader) history(ctx context.Context, userID string, at *time.Time, id *string, limit int) (HistoryPage, error) {
	rows, err := r.q.ListUserMatches(ctx, dbgen.ListUserMatchesParams{UserID: userID, CursorFinishedAt: at, CursorID: id, Lim: int32(limit + 1)})
	if err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Items: []HistoryItem{}}
	ids := make([]string, 0, len(rows))
	for i, row := range rows {
		if i == limit {
			last := page.Items[limit-1]
			page.NextCursor = pagination.Encode(*last.FinishedAt, last.ID)
			break
		}
		team := int(row.Seat) % 2
		item := HistoryItem{ID: row.ID, Status: replay.Status(row.Status), Team: team, CreatedAt: row.CreatedAt,
			FinishedAt: row.FinishedAt, Players: []MatchPlayer{}}
		if row.EndReason != nil {
			item.EndReason = replay.EndReason(*row.EndReason)
		}
		if row.WinnerTeam != nil {
			winner := int(*row.WinnerTeam)
			item.WinnerTeam, item.Won = &winner, winner == team
		}
		if row.ScoreA != nil && row.ScoreB != nil {
			item.Score = &[2]int{int(*row.ScoreA), int(*row.ScoreB)}
		}
		page.Items = append(page.Items, item)
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return page, nil
	}
	players, err := r.q.ListPlayersForMatches(ctx, ids)
	if err != nil {
		return HistoryPage{}, err
	}
	byMatch := make(map[string]*HistoryItem, len(page.Items))
	for i := range page.Items {
		byMatch[page.Items[i].ID] = &page.Items[i]
	}
	for _, p := range players {
		if it := byMatch[p.MatchID]; it != nil {
			it.Players = append(it.Players, MatchPlayer{Seat: int(p.Seat), Username: p.Username, DisplayName: p.DisplayName})
		}
	}
	return page, nil
}

// Directory busca pessoas por username ou nome de exibição (20 por página, ordem por username).
// Só contas ativas e com e-mail verificado; nunca devolve o e-mail.
func (r *Reader) Directory(ctx context.Context, query, cursor string) (DirectoryPage, error) {
	after, err := pagination.DecodeKey(cursor)
	if err != nil {
		return DirectoryPage{}, err
	}
	// Valida antes de normalizar: ToLower trocaria bytes inválidos por U+FFFD e esconderia o problema.
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) {
		return DirectoryPage{}, ErrInvalidQuery
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if utf8.RuneCountInString(query) > maxQueryRunes {
		return DirectoryPage{}, ErrInvalidQuery
	}
	rows, err := r.q.SearchDirectory(ctx, dbgen.SearchDirectoryParams{
		Query: likeEscaper.Replace(query), After: after, Lim: directoryPageSize + 1})
	if err != nil {
		return DirectoryPage{}, err
	}
	page := DirectoryPage{Items: []DirectoryEntry{}}
	for i, row := range rows {
		if i == directoryPageSize {
			page.NextCursor = pagination.EncodeKey(page.Items[directoryPageSize-1].Username)
			break
		}
		page.Items = append(page.Items, DirectoryEntry{Username: row.Username, DisplayName: row.DisplayName})
	}
	return page, nil
}
