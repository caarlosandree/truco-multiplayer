// Package mail envia e-mails transacionais (verificação, reset de senha, avisos).
package mail

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"sync"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// LogMailer escreve o e-mail no log. Útil para quem roda sem chave do Resend.
type LogMailer struct{ log *slog.Logger }

func NewLog(logger *slog.Logger) *LogMailer { return &LogMailer{log: logger} }

func (l *LogMailer) Send(ctx context.Context, m Message) error {
	l.log.InfoContext(ctx, "e-mail (MAIL_DRIVER=log)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}

// Fake guarda as mensagens em memória para os testes.
type Fake struct {
	mu   sync.Mutex
	sent []Message
	Err  error
}

func (f *Fake) Send(_ context.Context, m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *Fake) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

func (f *Fake) LastTo(to string) (Message, bool) {
	sent := f.Sent()
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].To == to {
			return sent[i], true
		}
	}
	return Message{}, false
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// LastToken extrai o token do último e-mail enviado para `to` ("" se não houver).
func (f *Fake) LastToken(to string) string {
	m, ok := f.LastTo(to)
	if !ok {
		return ""
	}
	match := tokenPattern.FindStringSubmatch(m.Text)
	if match == nil {
		return ""
	}
	return match[1]
}
