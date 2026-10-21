package lobby

import "time"

// Tetos de e-mail de convite. Passou do teto, o convite continua valendo só no app.
const (
	inviteMailsPerInviter = 10 // por quem convida, por hora
	inviteMailsPerInvitee = 3  // por convidado, por dia
)

// mailQuota conta envios numa janela deslizante por chave. Só é tocado com Lobby.mu travado.
type mailQuota struct {
	limit  int
	window time.Duration
	sent   map[string][]time.Time
}

func newMailQuota(limit int, window time.Duration) *mailQuota {
	return &mailQuota{limit: limit, window: window, sent: map[string][]time.Time{}}
}

// prune descarta os envios que já saíram da janela; chave sem envios some do mapa.
func (q *mailQuota) prune(key string, now time.Time) {
	cutoff := now.Add(-q.window)
	kept := q.sent[key][:0]
	for _, at := range q.sent[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(q.sent, key)
		return
	}
	q.sent[key] = kept
}

func (q *mailQuota) allows(key string, now time.Time) bool {
	q.prune(key, now)
	return len(q.sent[key]) < q.limit
}

func (q *mailQuota) record(key string, now time.Time) {
	q.prune(key, now)
	q.sent[key] = append(q.sent[key], now)
}
