// Package mail email iberýär. SMTP sazlanmadyk bolsa, haty loga ýazýar (dev).
package mail

import (
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"strings"
	"time"

	"github.com/Esca6585dev/os-astu-tm/api/internal/config"
)

type Mailer struct {
	cfg config.Config
}

func New(cfg config.Config) *Mailer { return &Mailer{cfg: cfg} }

func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg.SMTPHost == "" {
		slog.Info("EMAIL (SMTP sazlanmadyk, diňe log)", "to", to, "subject", subject, "body", body)
		return nil
	}
	headers := []string{
		"From: " + m.cfg.MailFrom,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + body
	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, m.cfg.MailFrom, []string{to}, []byte(msg))
}
