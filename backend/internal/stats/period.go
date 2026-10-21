package stats

import (
	"errors"
	"time"
	_ "time/tzdata" // o fuso de Brasília não pode depender do tzdata do sistema
)

// ErrInvalidRankingQuery indica período ou métrica desconhecidos.
var ErrInvalidRankingQuery = errors.New("stats: consulta de ranking inválida")

// Period é a janela de tempo do ranking.
type Period string

const (
	PeriodAll   Period = "all"
	PeriodMonth Period = "month"
	PeriodWeek  Period = "week"
)

// Metric é o critério de ordenação do ranking.
type Metric string

const (
	MetricWins    Metric = "wins"
	MetricWinRate Metric = "winrate"
)

var saoPaulo = mustLoadSaoPaulo()

func mustLoadSaoPaulo() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic("stats: fuso America/Sao_Paulo indisponível: " + err.Error())
	}
	return loc
}

// PeriodStart devolve o início da janela (segunda 00:00 ou dia 1 00:00 em
// America/Sao_Paulo) ou nil para o período geral.
func PeriodStart(p Period, now time.Time) *time.Time {
	local := now.In(saoPaulo)
	var start time.Time
	switch p {
	case PeriodWeek:
		back := (int(local.Weekday()) + 6) % 7 // segunda = 0
		start = time.Date(local.Year(), local.Month(), local.Day()-back, 0, 0, 0, 0, saoPaulo)
	case PeriodMonth:
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, saoPaulo)
	default:
		return nil
	}
	return &start
}
