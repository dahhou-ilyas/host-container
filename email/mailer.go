package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"os"
)

// Mailer sends transactional emails via SMTP.
// When SMTP_HOST is empty it prints links to stdout (zero-config for local dev).
type Mailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

func NewMailer() *Mailer {
	return &Mailer{
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		user:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     os.Getenv("SMTP_FROM"),
	}
}

func (m *Mailer) Enabled() bool { return m.host != "" }

func (m *Mailer) Send(to, subject, htmlBody string) error {
	if !m.Enabled() {
		fmt.Printf("[EMAIL dev] To: %s | Subject: %s\n%s\n---\n", to, subject, htmlBody)
		return nil
	}
	auth := smtp.PlainAuth("", m.user, m.password, m.host)
	msg := buildMessage(m.from, to, subject, htmlBody)
	addr := m.host + ":" + m.port
	if m.port == "465" {
		return sendTLS(addr, auth, m.from, to, msg)
	}
	return smtp.SendMail(addr, auth, m.from, []string{to}, msg)
}

func (m *Mailer) SendVerificationEmail(to, name, verifyURL string) error {
	html := renderTemplate(verificationTmpl, map[string]string{"Name": name, "URL": verifyURL})
	return m.Send(to, "Verify your Docker Wrapper account", html)
}

func (m *Mailer) SendPasswordResetEmail(to, name, resetURL string) error {
	html := renderTemplate(passwordResetTmpl, map[string]string{"Name": name, "URL": resetURL})
	return m.Send(to, "Reset your Docker Wrapper password", html)
}

func buildMessage(from, to, subject, htmlBody string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from, to, subject)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(htmlBody)
	return b.Bytes()
}

func sendTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	host, _, _ := net.SplitHostPort(addr)
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Quit()
	if err = c.Auth(auth); err != nil {
		return err
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	w.Close()
	return err
}

func renderTemplate(tmpl string, data map[string]string) string {
	t := template.Must(template.New("").Parse(tmpl))
	var b bytes.Buffer
	t.Execute(&b, data)
	return b.String()
}
