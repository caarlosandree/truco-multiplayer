package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ResendMailer chama a API HTTP do Resend diretamente (um único POST, sem SDK).
type ResendMailer struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
	backoff  time.Duration
}

func NewResend(apiKey, from string) *ResendMailer {
	return &ResendMailer{apiKey: apiKey, from: from, endpoint: "https://api.resend.com/emails",
		client: &http.Client{Timeout: 10 * time.Second}, backoff: 500 * time.Millisecond}
}

// Send tenta até 3 vezes em erro de rede, 429 ou 5xx.
func (r *ResendMailer) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(map[string]any{"from": r.from, "to": []string{m.To}, "subject": m.Subject, "text": m.Text, "html": m.HTML})
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.backoff << (attempt - 1)):
			}
		}
		retry, err := r.post(ctx, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
	}
	return lastErr
}

func (r *ResendMailer) post(ctx context.Context, body []byte) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return true, fmt.Errorf("mail: falha de rede ao chamar o Resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return false, nil
	}
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	err = fmt.Errorf("mail: Resend respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500, err
}
