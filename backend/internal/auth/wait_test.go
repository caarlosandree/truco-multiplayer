package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

type blockingMailer struct{ release chan struct{} }

func (m blockingMailer) Send(context.Context, mail.Message) error {
	<-m.release
	return nil
}

func TestWaitBlocksUntilBackgroundSendsFinish(t *testing.T) {
	release := make(chan struct{})
	s := NewService(Options{Mailer: blockingMailer{release}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s.sendLater(mail.Message{To: "ana@empresa.com"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait com envio pendente = %v", err)
	}
	close(release)
	if err := s.Wait(context.Background()); err != nil {
		t.Fatalf("Wait depois do envio = %v", err)
	}
}
