package stats_test

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestPeriodStartBoundaries(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		now    time.Time
		period stats.Period
		want   *time.Time
	}{
		{"segunda 00:00 é o próprio início da semana", time.Date(2026, 10, 19, 0, 0, 0, 0, sp), stats.PeriodWeek, ptr(time.Date(2026, 10, 19, 0, 0, 0, 0, sp))},
		{"domingo 23:30 de Brasília ainda é a semana anterior", time.Date(2026, 10, 19, 2, 30, 0, 0, time.UTC), stats.PeriodWeek, ptr(time.Date(2026, 10, 12, 0, 0, 0, 0, sp))},
		{"31/10 23:59 de Brasília ainda é outubro", time.Date(2026, 11, 1, 2, 59, 0, 0, time.UTC), stats.PeriodMonth, ptr(time.Date(2026, 10, 1, 0, 0, 0, 0, sp))},
		{"semana em horário de verão", time.Date(2018, 11, 7, 12, 0, 0, 0, time.FixedZone("-02", -2*3600)), stats.PeriodWeek, ptr(time.Date(2018, 11, 5, 0, 0, 0, 0, sp))},
		{"dia seguinte ao fim do verão", time.Date(2019, 2, 18, 10, 0, 0, 0, time.FixedZone("-03", -3*3600)), stats.PeriodWeek, ptr(time.Date(2019, 2, 18, 0, 0, 0, 0, sp))},
		{"geral não tem início", time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), stats.PeriodAll, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stats.PeriodStart(c.period, c.now)
			if c.want == nil {
				if got != nil {
					t.Fatalf("got %v, queria nil", got)
				}
				return
			}
			if got == nil || !got.Equal(*c.want) {
				t.Fatalf("got %v, queria %v", got, c.want)
			}
		})
	}
	// Sanidade dos offsets que a tabela afirma (-02 em 2018-11-05, -03 em 2019-02-18).
	if _, off := time.Date(2018, 11, 5, 0, 0, 0, 0, sp).Zone(); off != -2*3600 {
		t.Fatalf("offset de 05/11/2018 = %d", off)
	}
}

func ptr[T any](v T) *T { return &v }
