// Package config carrega a configuração do servidor a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// MailDriver escolhe como os e-mails são enviados.
type MailDriver string

const (
	// MailResend envia pela API da Resend.
	MailResend MailDriver = "resend"
	// MailLog escreve os e-mails no log, sem envio real.
	MailLog MailDriver = "log"
)

// GameTiming reúne os tempos das salas (Spec 3 §12). Tem os mesmos campos, na mesma
// ordem, de room.Timing, para permitir a conversão direta room.Timing(cfg.Game).
type GameTiming struct {
	Play, Vote, Choice, Burn, Absent time.Duration
	HandPause                        time.Duration
	AbsenceGrace, AbsenceBudget      time.Duration
	WaitingDisconnect                time.Duration
}

// BotConfig reúne os parâmetros dos bots (Spec 5 §10).
type BotConfig struct {
	ThinkBudget, DelayMin, DelayMax time.Duration
	MaxConcurrent                   int
}

// Config reúne as opções do servidor já validadas.
type Config struct {
	DatabaseURL         string
	HTTPAddr            string
	AppBaseURL          string // sem barra final
	AppOrigin           string // esquema + host, usado na checagem de Origin
	CookieSecure        bool
	TrustProxy          bool
	AllowedEmailDomains []string
	MailDriver          MailDriver
	ResendAPIKey        string
	MailFrom            string
	RankingMinMatches   int
	MigrateOnStart      bool
	LogLevel            slog.Level
	LogFormat           string
	StaticDir           string
	Game                GameTiming
	Bots                BotConfig
	MaxRooms            int
	ShutdownTimeout     time.Duration
}

// Load lê e valida a configuração; devolve todos os problemas de uma vez.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	get := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	parseBool := func(key string, fallback bool) bool {
		raw := get(key, "")
		if raw == "" {
			return fallback
		}
		v, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: valor booleano inválido %q", key, raw))
			return fallback
		}
		return v
	}

	cfg := Config{
		DatabaseURL:    get("DATABASE_URL", ""),
		HTTPAddr:       get("HTTP_ADDR", ":8080"),
		CookieSecure:   parseBool("COOKIE_SECURE", true),
		TrustProxy:     parseBool("TRUST_PROXY", false),
		MailDriver:     MailDriver(get("MAIL_DRIVER", string(MailResend))),
		ResendAPIKey:   get("RESEND_API_KEY", ""),
		MailFrom:       get("MAIL_FROM", ""),
		MigrateOnStart: parseBool("MIGRATE_ON_START", true),
		LogFormat:      get("LOG_FORMAT", "json"),
		StaticDir:      get("STATIC_DIR", ""),
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL é obrigatória"))
	}

	base, origin, err := parseBaseURL(get("APP_BASE_URL", ""))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AppBaseURL, cfg.AppOrigin = base, origin

	for _, d := range strings.Split(get("ALLOWED_EMAIL_DOMAINS", ""), ",") {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
		if d != "" {
			cfg.AllowedEmailDomains = append(cfg.AllowedEmailDomains, d)
		}
	}

	switch cfg.MailDriver {
	case MailResend:
		if cfg.ResendAPIKey == "" {
			errs = append(errs, errors.New("RESEND_API_KEY é obrigatória com MAIL_DRIVER=resend"))
		}
		if cfg.MailFrom == "" {
			errs = append(errs, errors.New("MAIL_FROM é obrigatória com MAIL_DRIVER=resend"))
		}
	case MailLog:
	default:
		errs = append(errs, fmt.Errorf("MAIL_DRIVER: use resend ou log, não %q", cfg.MailDriver))
	}

	minMatches, err := strconv.Atoi(get("RANKING_MIN_MATCHES", "5"))
	if err != nil || minMatches < 1 {
		errs = append(errs, errors.New("RANKING_MIN_MATCHES deve ser um inteiro >= 1"))
	}
	cfg.RankingMinMatches = minMatches

	intIn := func(key string, fallback, lo, hi int) int {
		raw := get(key, strconv.Itoa(fallback))
		v, err := strconv.Atoi(raw)
		if err != nil || v < lo || v > hi {
			errs = append(errs, fmt.Errorf("%s deve ser um inteiro entre %d e %d, não %q", key, lo, hi, raw))
			return fallback
		}
		return v
	}
	seconds := func(key string, fallback, lo, hi int) time.Duration {
		return time.Duration(intIn(key, fallback, lo, hi)) * time.Second
	}
	cfg.Game = GameTiming{
		Play:              seconds("TIMER_PLAY_SECONDS", 30, 5, 300),
		Vote:              seconds("TIMER_VOTE_SECONDS", 20, 5, 300),
		Choice:            seconds("TIMER_CHOICE_SECONDS", 15, 5, 300),
		Burn:              seconds("TIMER_BURN_SECONDS", 15, 5, 300),
		Absent:            seconds("TIMER_ABSENT_SECONDS", 5, 1, 60),
		HandPause:         seconds("HAND_PAUSE_SECONDS", 4, 0, 30),
		AbsenceGrace:      seconds("ABSENCE_GRACE_SECONDS", 120, 10, 3600),
		AbsenceBudget:     seconds("ABSENCE_BUDGET_SECONDS", 300, 10, 7200),
		WaitingDisconnect: seconds("WAITING_DISCONNECT_SECONDS", 60, 5, 3600),
	}
	if cfg.Game.AbsenceBudget < cfg.Game.AbsenceGrace {
		errs = append(errs, errors.New("ABSENCE_BUDGET_SECONDS deve ser >= ABSENCE_GRACE_SECONDS"))
	}
	millis := func(key string, fallback, lo, hi int) time.Duration {
		return time.Duration(intIn(key, fallback, lo, hi)) * time.Millisecond
	}
	cfg.Bots = BotConfig{
		ThinkBudget:   millis("BOT_THINK_BUDGET_MS", 800, 50, 5000),
		MaxConcurrent: intIn("BOT_MAX_CONCURRENT", min(max(runtime.GOMAXPROCS(0), 1), 64), 1, 64),
		DelayMin:      millis("BOT_DELAY_MIN_MS", 1000, 0, 10000),
		DelayMax:      millis("BOT_DELAY_MAX_MS", 3000, 0, 10000),
	}
	if cfg.Bots.DelayMax < cfg.Bots.DelayMin {
		errs = append(errs, errors.New("BOT_DELAY_MAX_MS deve ser >= BOT_DELAY_MIN_MS"))
	}
	cfg.MaxRooms = intIn("MAX_ROOMS", 50, 1, 1000)
	cfg.ShutdownTimeout = seconds("SHUTDOWN_TIMEOUT_SECONDS", 15, 1, 120)

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL inválido: %v", err))
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT: use json ou text, não %q", cfg.LogFormat))
	}
	return cfg, errors.Join(errs...)
}

// parseBaseURL valida APP_BASE_URL e devolve a base sem barra final e a origem.
func parseBaseURL(raw string) (base, origin string, err error) {
	if raw == "" {
		return "", "", errors.New("APP_BASE_URL é obrigatória")
	}
	u, parseErr := url.Parse(raw)
	if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", fmt.Errorf("APP_BASE_URL deve ser uma URL http(s) absoluta, não %q", raw)
	}
	if strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("APP_BASE_URL não pode ter caminho, query ou fragmento: %q", raw)
	}
	// Navegadores mandam Origin com host em minúsculas e sem a porta padrão do esquema.
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	origin = u.Scheme + "://" + host
	return origin, origin, nil
}
