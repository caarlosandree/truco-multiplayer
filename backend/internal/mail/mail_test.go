package mail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestResend(t *testing.T, handler http.HandlerFunc) *ResendMailer {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m := NewResend("re_teste", "Truco <truco@empresa.com>")
	m.endpoint, m.backoff = srv.URL, time.Millisecond
	return m
}

func TestResendSendsExpectedRequest(t *testing.T) {
	var got map[string]any
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer re_teste" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("requisição inesperada: %s %v", r.Method, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})
	err := m.Send(context.Background(), Message{To: "ana@empresa.com", Subject: "Oi", Text: "texto", HTML: "<p>html</p>"})
	if err != nil {
		t.Fatal(err)
	}
	to, _ := got["to"].([]any)
	if got["from"] != "Truco <truco@empresa.com>" || len(to) != 1 || to[0] != "ana@empresa.com" || got["subject"] != "Oi" || got["text"] != "texto" || got["html"] != "<p>html</p>" {
		t.Fatalf("corpo = %v", got)
	}
}

func TestResendRetriesServerErrors(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := m.Send(context.Background(), Message{To: "a@b.com"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("%d chamadas, quer 3", calls.Load())
	}
}

func TestResendDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"from inválido"}`))
	})
	err := m.Send(context.Background(), Message{To: "a@b.com"})
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "from inválido") {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d chamadas, quer 1", calls.Load())
	}
}

func TestResendGivesUpAfterThreeAttempts(t *testing.T) {
	var calls atomic.Int32
	m := newTestResend(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if err := m.Send(context.Background(), Message{To: "a@b.com"}); err == nil {
		t.Fatal("esperava erro")
	}
	if calls.Load() != 3 {
		t.Fatalf("%d chamadas, quer 3", calls.Load())
	}
}

func TestFakeCapturesAndExtractsToken(t *testing.T) {
	f := &Fake{}
	_ = f.Send(context.Background(), VerifyEmail("ana@empresa.com", "Ana", "https://truco.test/verificar-email?token=AbC_-123"))
	if got := f.LastToken("ana@empresa.com"); got != "AbC_-123" {
		t.Fatalf("token = %q", got)
	}
	if _, ok := f.LastTo("bia@empresa.com"); ok {
		t.Fatal("não houve e-mail para bia")
	}
	f.Err = errors.New("falhou")
	if err := f.Send(context.Background(), Message{To: "x"}); err == nil {
		t.Fatal("Fake.Err deve ser devolvido")
	}
}

func TestTemplatesRenderLinksAndEscapeHTML(t *testing.T) {
	msgs := []Message{
		VerifyEmail("a@b.com", "<script>", "https://t/verificar-email?token=x"),
		AccountExists("a@b.com", "<script>", "https://t/entrar", "https://t/esqueci-senha"),
		PasswordReset("a@b.com", "<script>", "https://t/redefinir-senha?token=x"),
		ForcedPasswordReset("a@b.com", "<script>", "https://t/redefinir-senha?token=x"),
		PasswordChanged("a@b.com", "<script>"),
	}
	for _, m := range msgs {
		if m.To != "a@b.com" || m.Subject == "" || m.Text == "" || m.HTML == "" {
			t.Fatalf("mensagem incompleta: %+v", m)
		}
		if strings.Contains(m.HTML, "<script>") {
			t.Fatalf("HTML sem escape: %s", m.HTML)
		}
	}
	if !strings.Contains(msgs[0].Text, "https://t/verificar-email?token=x") || !strings.Contains(msgs[0].HTML, "https://t/verificar-email?token=x") {
		t.Fatal("link ausente na verificação")
	}
	if !strings.Contains(msgs[1].Text, "https://t/esqueci-senha") {
		t.Fatal("link de reset ausente em AccountExists")
	}
}
