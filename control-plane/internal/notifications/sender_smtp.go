package notifications

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type SMTPSender struct {
	host     string
	port     int
	from     string
	username string
	password string
}

func NewSMTPSender(host string, port int, fromEmail, username, password string) (*SMTPSender, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("SMTP_HOST is required for smtp notification backend")
	}
	if port <= 0 {
		return nil, errors.New("SMTP_PORT must be > 0 for smtp notification backend")
	}
	fromEmail = strings.TrimSpace(fromEmail)
	if fromEmail == "" {
		return nil, errors.New("SMTP_FROM_EMAIL is required for smtp notification backend")
	}
	return &SMTPSender{
		host:     host,
		port:     port,
		from:     fromEmail,
		username: strings.TrimSpace(username),
		password: password,
	}, nil
}

func (s *SMTPSender) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	if to == "" {
		return errors.New("recipient email is required")
	}
	if subject == "" {
		return errors.New("subject is required")
	}
	if textBody == "" {
		textBody = "Please open this email in an HTML-capable client."
	}
	addr := s.host + ":" + strconv.Itoa(s.port)
	message := buildMultipartMessage(s.from, to, subject, textBody, htmlBody)
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	if err := smtp.SendMail(addr, auth, s.from, []string{to}, message); err != nil {
		return fmt.Errorf("smtp send mail: %w", err)
	}
	return nil
}

func buildMultipartMessage(from, to, subject, textBody, htmlBody string) []byte {
	boundary := "nexus-alt-boundary"
	var b bytes.Buffer
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(textBody + "\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(htmlBody + "\r\n")
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes()
}
