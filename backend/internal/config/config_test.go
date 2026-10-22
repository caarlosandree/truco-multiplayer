package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
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

func TestGameTimingDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	want := GameTiming{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second, Burn: 15 * time.Second,
		Absent: 5 * time.Second, HandPause: 4 * time.Second, AbsenceGrace: 120 * time.Second,
		AbsenceBudget: 300 * time.Second, WaitingDisconnect: 60 * time.Second}
	if cfg.Game != want || cfg.MaxRooms != 50 || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("padrões: %+v, MaxRooms %d, Shutdown %v", cfg.Game, cfg.MaxRooms, cfg.ShutdownTimeout)
	}
}

func TestGameTimingOverridesAndLimits(t *testing.T) {
	values := validEnv()
	values["TIMER_PLAY_SECONDS"] = "45"
	values["HAND_PAUSE_SECONDS"] = "0"
	values["MAX_ROOMS"] = "7"
	cfg, err := Load(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Game.Play != 45*time.Second || cfg.Game.HandPause != 0 || cfg.MaxRooms != 7 {
		t.Fatalf("sobrescritas: %+v %d", cfg.Game, cfg.MaxRooms)
	}
	for key, bad := range map[string]string{
		"TIMER_PLAY_SECONDS": "4", "TIMER_VOTE_SECONDS": "301", "TIMER_CHOICE_SECONDS": "x",
		"TIMER_BURN_SECONDS": "0", "TIMER_ABSENT_SECONDS": "61", "HAND_PAUSE_SECONDS": "31",
		"ABSENCE_GRACE_SECONDS": "9", "WAITING_DISCONNECT_SECONDS": "4", "MAX_ROOMS": "0",
		"SHUTDOWN_TIMEOUT_SECONDS": "121",
	} {
		values := validEnv()
		values[key] = bad
		if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s=%s deveria falhar citando a variável: %v", key, bad, err)
		}
	}
}

func TestAbsenceBudgetMustCoverGrace(t *testing.T) {
	values := validEnv()
	values["ABSENCE_GRACE_SECONDS"] = "200"
	values["ABSENCE_BUDGET_SECONDS"] = "100"
	if _, err := Load(env(values)); err == nil || !strings.Contains(err.Error(), "ABSENCE_BUDGET_SECONDS") {
		t.Fatalf("orçamento menor que a carência deve falhar: %v", err)
	}
}

func TestBotConfigDefaults(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	b := cfg.Bots
	if b.ThinkBudget != 800*time.Millisecond || b.DelayMin != time.Second || b.DelayMax != 3*time.Second {
		t.Fatalf("padrões dos bots errados: %+v", b)
	}
	if b.MaxConcurrent < 1 || b.MaxConcurrent > 64 {
		t.Fatalf("MaxConcurrent = %d fora de 1..64", b.MaxConcurrent)
	}
}

func TestBotConfigOverridesAndLimits(t *testing.T) {
	with := func(k, v string) map[string]string {
		m := validEnv()
		m[k] = v
		return m
	}
	cfg, err := Load(env(func() map[string]string {
		m := validEnv()
		m["BOT_THINK_BUDGET_MS"] = "50"
		m["BOT_MAX_CONCURRENT"] = "64"
		m["BOT_DELAY_MIN_MS"] = "0"
		m["BOT_DELAY_MAX_MS"] = "10000"
		return m
	}()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bots.ThinkBudget != 50*time.Millisecond || cfg.Bots.MaxConcurrent != 64 || cfg.Bots.DelayMin != 0 || cfg.Bots.DelayMax != 10*time.Second {
		t.Fatalf("bots = %+v", cfg.Bots)
	}
	for _, c := range [][2]string{
		{"BOT_THINK_BUDGET_MS", "49"}, {"BOT_THINK_BUDGET_MS", "5001"},
		{"BOT_MAX_CONCURRENT", "0"}, {"BOT_MAX_CONCURRENT", "65"},
		{"BOT_DELAY_MIN_MS", "-1"}, {"BOT_DELAY_MIN_MS", "10001"},
		{"BOT_DELAY_MAX_MS", "10001"}, {"BOT_DELAY_MAX_MS", "abc"},
	} {
		if _, err := Load(env(with(c[0], c[1]))); err == nil || !strings.Contains(err.Error(), c[0]) {
			t.Errorf("%s=%s deveria falhar citando a variável, err = %v", c[0], c[1], err)
		}
	}
}

func TestBotDelayMaxMustCoverMin(t *testing.T) {
	m := validEnv()
	m["BOT_DELAY_MIN_MS"] = "2000"
	m["BOT_DELAY_MAX_MS"] = "1000"
	if _, err := Load(env(m)); err == nil || !strings.Contains(err.Error(), "BOT_DELAY_MAX_MS") {
		t.Fatalf("err = %v", err)
	}
}
