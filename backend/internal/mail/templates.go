package mail

import (
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

type templateData struct {
	Name, Link, LoginLink, ResetLink string
	Inviter, Room                    string
}

type template struct {
	subject *texttemplate.Template
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

func newTemplate(subject, text, html string) template {
	return template{
		subject: texttemplate.Must(texttemplate.New("subject").Parse(subject)),
		text:    texttemplate.Must(texttemplate.New("text").Parse(text)),
		html:    htmltemplate.Must(htmltemplate.New("html").Parse(html)),
	}
}

func (t template) render(to string, data templateData) Message {
	var subject, text, html strings.Builder
	if err := t.subject.Execute(&subject, data); err != nil {
		panic(err)
	}
	// Os templates são fixos e testados; erro aqui é bug de programação.
	if err := t.text.Execute(&text, data); err != nil {
		panic(err)
	}
	if err := t.html.Execute(&html, data); err != nil {
		panic(err)
	}
	return Message{To: to, Subject: headerSafe(subject.String()), Text: text.String(), HTML: html.String()}
}

// headerSafe tira quebras de linha do assunto: o nome vem do usuário e não pode injetar cabeçalhos.
func headerSafe(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var (
	verifyEmailTemplate = newTemplate("Confirme seu e-mail no Truco",
		"Olá, {{.Name}}!\n\nConfirme seu e-mail para ativar sua conta no Truco:\n{{.Link}}\n\nO link vale por 24 horas. Se você não pediu este cadastro, ignore esta mensagem.\n",
		`<p>Olá, {{.Name}}!</p><p>Confirme seu e-mail para ativar sua conta no Truco:</p><p><a href="{{.Link}}">Confirmar e-mail</a></p><p>O link vale por 24 horas. Se você não pediu este cadastro, ignore esta mensagem.</p>`)

	accountExistsTemplate = newTemplate("Você já tem uma conta no Truco",
		"Olá, {{.Name}}!\n\nAlguém tentou criar uma conta no Truco com este e-mail, mas ele já está cadastrado.\nPara entrar: {{.LoginLink}}\nEsqueceu a senha? {{.ResetLink}}\n\nSe não foi você, ignore esta mensagem.\n",
		`<p>Olá, {{.Name}}!</p><p>Alguém tentou criar uma conta no Truco com este e-mail, mas ele já está cadastrado.</p><p><a href="{{.LoginLink}}">Entrar</a> · <a href="{{.ResetLink}}">Esqueci minha senha</a></p><p>Se não foi você, ignore esta mensagem.</p>`)

	passwordResetTemplate = newTemplate("Redefinição de senha do Truco",
		"Olá, {{.Name}}!\n\nPara criar uma nova senha, acesse:\n{{.Link}}\n\nO link vale por 30 minutos. Se você não pediu, ignore esta mensagem: sua senha continua a mesma.\n",
		`<p>Olá, {{.Name}}!</p><p>Para criar uma nova senha, acesse:</p><p><a href="{{.Link}}">Redefinir senha</a></p><p>O link vale por 30 minutos. Se você não pediu, ignore esta mensagem: sua senha continua a mesma.</p>`)

	forcedResetTemplate = newTemplate("Você precisa redefinir sua senha do Truco",
		"Olá, {{.Name}}!\n\nUm administrador pediu que você redefina sua senha. Suas sessões foram encerradas.\nCrie uma nova senha em:\n{{.Link}}\n\nO link vale por 30 minutos; depois disso, use \"Esqueci minha senha\".\n",
		`<p>Olá, {{.Name}}!</p><p>Um administrador pediu que você redefina sua senha. Suas sessões foram encerradas.</p><p><a href="{{.Link}}">Criar nova senha</a></p><p>O link vale por 30 minutos; depois disso, use "Esqueci minha senha".</p>`)

	passwordChangedTemplate = newTemplate("Sua senha do Truco foi alterada",
		"Olá, {{.Name}}!\n\nA senha da sua conta no Truco acabou de ser alterada e as outras sessões foram encerradas.\nSe não foi você, use \"Esqueci minha senha\" imediatamente.\n",
		`<p>Olá, {{.Name}}!</p><p>A senha da sua conta no Truco acabou de ser alterada e as outras sessões foram encerradas.</p><p>Se não foi você, use "Esqueci minha senha" imediatamente.</p>`)

	roomInviteTemplate = newTemplate("{{.Inviter}} te chamou pra mesa no Truco",
		"Olá!\n\n{{.Inviter}} te chamou pra mesa \"{{.Room}}\" no Truco.\nPara entrar:\n{{.Link}}\n\nO convite vale enquanto a sala tiver lugar. Este link é só seu: entre com a sua conta.\n",
		`<p>Olá!</p><p>{{.Inviter}} te chamou pra mesa <strong>{{.Room}}</strong> no Truco.</p><p><a href="{{.Link}}">Entrar na mesa</a></p><p>O convite vale enquanto a sala tiver lugar. Este link é só seu: entre com a sua conta.</p>`)
)

func VerifyEmail(to, name, link string) Message {
	return verifyEmailTemplate.render(to, templateData{Name: name, Link: link})
}

func AccountExists(to, name, loginLink, resetLink string) Message {
	return accountExistsTemplate.render(to, templateData{Name: name, LoginLink: loginLink, ResetLink: resetLink})
}

func PasswordReset(to, name, link string) Message {
	return passwordResetTemplate.render(to, templateData{Name: name, Link: link})
}

func ForcedPasswordReset(to, name, link string) Message {
	return forcedResetTemplate.render(to, templateData{Name: name, Link: link})
}

func PasswordChanged(to, name string) Message {
	return passwordChangedTemplate.render(to, templateData{Name: name})
}

// RoomInvite avisa quem estava offline que foi chamado para uma sala; o link leva o token do convite.
func RoomInvite(to, inviterName, roomName, link string) Message {
	return roomInviteTemplate.render(to, templateData{Inviter: inviterName, Room: roomName, Link: link})
}
