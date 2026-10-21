package lobby

import (
	"testing"
	"time"
)

func TestMailQuotaSlidingWindow(t *testing.T) {
	q := newMailQuota(3, time.Hour)
	t0 := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for i := range 3 {
		now := t0.Add(time.Duration(i) * time.Minute)
		if !q.allows("ana", now) {
			t.Fatalf("envio %d deveria caber", i+1)
		}
		q.record("ana", now)
	}
	if q.allows("ana", t0.Add(59*time.Minute)) {
		t.Fatal("quarto envio dentro da janela deveria ser barrado")
	}
	if !q.allows("bia", t0.Add(59*time.Minute)) {
		t.Fatal("o teto é por chave")
	}
	if !q.allows("ana", t0.Add(time.Hour)) {
		t.Fatal("o primeiro envio saiu da janela de 1 h; deveria caber de novo")
	}
	q.record("ana", t0.Add(3*time.Hour))
	if got := len(q.sent["ana"]); got != 1 {
		t.Fatalf("envios antigos deveriam ser podados: %d", got)
	}
}
