package auth

import (
	netmail "net/mail"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.]{3,20}$`)

func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func NormalizeEmail(s string) string    { return strings.ToLower(strings.TrimSpace(s)) }

// FieldErrors mapeia campo → motivo e é devolvido como erro de validação.
type FieldErrors map[string]string

func (f FieldErrors) Error() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + f[k]
	}
	return "auth: dados inválidos (" + strings.Join(parts, "; ") + ")"
}

func ValidateUsername(u string) string {
	if !usernamePattern.MatchString(u) {
		return "use de 3 a 20 caracteres: letras minúsculas, números, ponto ou sublinhado"
	}
	return ""
}

// ValidateEmail espera o e-mail já normalizado e devolve seu domínio.
func ValidateEmail(e string) (domain, msg string) {
	addr, err := netmail.ParseAddress(e)
	if err != nil || addr.Name != "" || addr.Address != e {
		return "", "e-mail inválido"
	}
	at := strings.LastIndex(e, "@")
	if at < 1 || at == len(e)-1 {
		return "", "e-mail inválido"
	}
	return e[at+1:], ""
}

func ValidateDisplayName(n string) (string, string) {
	n = strings.TrimSpace(n)
	count := utf8.RuneCountInString(n)
	if count < 1 || count > 30 {
		return "", "o nome de exibição deve ter de 1 a 30 caracteres"
	}
	if strings.ContainsFunc(n, unicode.IsControl) {
		return "", "o nome de exibição não pode ter caracteres de controle"
	}
	return n, ""
}

func ValidatePassword(pw, username, email string) string {
	length := utf8.RuneCountInString(pw)
	if length < 10 || length > 128 {
		return "a senha deve ter de 10 a 128 caracteres"
	}
	lower := strings.ToLower(pw)
	if len(username) >= 3 && strings.Contains(lower, strings.ToLower(username)) {
		return "a senha não pode conter o nome de usuário"
	}
	if at := strings.Index(email, "@"); at >= 3 && strings.Contains(lower, strings.ToLower(email[:at])) {
		return "a senha não pode conter o seu e-mail"
	}
	return ""
}
