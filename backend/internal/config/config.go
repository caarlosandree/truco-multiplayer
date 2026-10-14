// Package config carrega a configuração do servidor a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

// MailDriver escolhe como os e-mails são enviados.
type MailDriver string

const (
	// MailResend envia pela API da Resend.
	MailResend MailDriver = "resend"
	// MailLog escreve os e-mails no log, sem envio real.
	MailLog MailDriver = "log"
)

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
	origin = u.Scheme + "://" + u.Host
	return origin, origin, nil
}
