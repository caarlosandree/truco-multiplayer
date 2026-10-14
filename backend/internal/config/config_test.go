package config

import (
	"log/slog"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":          "postgres://truco:truco@localhost:5432/truco?sslmode=disable",
		"APP_BASE_URL":          "https://truco.empresa.com.br/",
		"ALLOWED_EMAIL_DOMAINS": " Empresa.com.br, @filial.com.br ,,",
		"RESEND_API_KEY":        "re_123",
		"MAIL_FROM":             "Truco <truco@empresa.com.br>",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || !cfg.CookieSecure || cfg.TrustProxy || !cfg.MigrateOnStart {
		t.Fatalf("padrões errados: %+v", cfg)
	}
	if cfg.AppBaseURL != "https://truco.empresa.com.br" || cfg.AppOrigin != "https://truco.empresa.com.br" {
		t.Fatalf("base %q origin %q", cfg.AppBaseURL, cfg.AppOrigin)
	}
	if strings.Join(cfg.AllowedEmailDomains, ",") != "empresa.com.br,filial.com.br" {
		t.Fatalf("domínios = %v", cfg.AllowedEmailDomains)
	}
	if cfg.MailDriver != MailResend || cfg.RankingMinMatches != 5 || cfg.LogLevel != slog.LevelInfo || cfg.LogFormat != "json" {
		t.Fatalf("padrões errados: %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	values := validEnv()
	values["HTTP_ADDR"] = "127.0.0.1:9000"
	values["COOKIE_SECURE"] = "false"
	values["TRUST_PROXY"] = "true"
	values["MAIL_DRIVER"] = "log"
	values["RESEND_API_KEY"] = ""
	values["MAIL_FROM"] = ""
	values["RANKING_MIN_MATCHES"] = "1"
	values["MIGRATE_ON_START"] = "false"
	values["LOG_LEVEL"] = "debug"
	values["LOG_FORMAT"] = "text"
	values["STATIC_DIR"] = "../frontend/dist"
	values["APP_BASE_URL"] = "http://localhost:5173"
	cfg, err := Load(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure || !cfg.TrustProxy || cfg.MailDriver != MailLog || cfg.RankingMinMatches != 1 ||
		cfg.MigrateOnStart || cfg.LogLevel != slog.LevelDebug || cfg.LogFormat != "text" ||
		cfg.StaticDir != "../frontend/dist" || cfg.AppOrigin != "http://localhost:5173" {
		t.Fatalf("overrides não aplicados: %+v", cfg)
	}
}

func TestLoadReportsAllProblems(t *testing.T) {
	values := map[string]string{
		"APP_BASE_URL":        "ftp://x",
		"COOKIE_SECURE":       "talvez",
		"MAIL_DRIVER":         "smtp",
		"RANKING_MIN_MATCHES": "0",
		"LOG_FORMAT":          "xml",
		"LOG_LEVEL":           "barulhento",
	}
	_, err := Load(env(values))
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, want := range []string{"DATABASE_URL", "APP_BASE_URL", "COOKIE_SECURE", "MAIL_DRIVER", "RANKING_MIN_MATCHES", "LOG_FORMAT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro não menciona %s: %v", want, err)
		}
	}
}

func TestLoadRequiresResendSettings(t *testing.T) {
	values := validEnv()
	values["RESEND_API_KEY"] = ""
	values["MAIL_FROM"] = ""
	_, err := Load(env(values))
	if err == nil || !strings.Contains(err.Error(), "RESEND_API_KEY") || !strings.Contains(err.Error(), "MAIL_FROM") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsBaseURLWithPath(t *testing.T) {
	values := validEnv()
	values["APP_BASE_URL"] = "https://empresa.com.br/truco"
	if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), "APP_BASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestAppOriginIsNormalized(t *testing.T) {
	cases := map[string]string{
		"https://Truco.Empresa.COM:443/": "https://truco.empresa.com",
		"http://Truco.Test:80":           "http://truco.test",
		"HTTP://localhost:5173":          "http://localhost:5173",
		"https://truco.test:8443":        "https://truco.test:8443",
		"http://truco.test:443":          "http://truco.test:443",
		"https://[::1]:443":              "https://[::1]",
		"https://[::1]:8443":             "https://[::1]:8443",
	}
	for raw, want := range cases {
		values := validEnv()
		values["APP_BASE_URL"] = raw
		cfg, err := Load(env(values))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if cfg.AppOrigin != want || cfg.AppBaseURL != want {
			t.Errorf("%s: base %q origin %q, quer %q", raw, cfg.AppBaseURL, cfg.AppOrigin, want)
		}
	}
}
