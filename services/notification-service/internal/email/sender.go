package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

type Sender interface {
	Send(to string, subject string, body string) error
}

// SMTPSender sends email via unauthenticated SMTP (Mailpit-compatible).
type SMTPSender struct {
	addr string
	from string
}

func NewSMTPSender(host string, port string, from string) *SMTPSender {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	from = strings.TrimSpace(from)
	if from == "" {
		from = "no-reply@apptremind.local"
	}
	return &SMTPSender{
		addr: fmt.Sprintf("%s:%s", host, port),
		from: from,
	}
}

func (s *SMTPSender) Send(to string, subject string, body string) error {
	msg := buildMessage(s.from, to, subject, body)
	return smtp.SendMail(s.addr, nil, s.from, []string{to}, []byte(msg))
}

func buildMessage(from, to, subject, body string) string {
	// Minimal RFC 5322 message; enough for Mailpit and most SMTP relays.
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		from,
		to,
		subject,
		body,
	)
}
