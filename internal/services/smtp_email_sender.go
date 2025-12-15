package services

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

type SMTPSender struct {
	Host string
	Port string
	User string
	Pass string
	From string
	UseTLS bool
}

func (s *SMTPSender) Send(to string, subject string, body string) error {
	addr := net.JoinHostPort(s.Host, s.Port)

	contentType := "text/plain; charset=\"utf-8\""
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "<") && (strings.Contains(trimmed, "<html") || strings.Contains(trimmed, "<body") || strings.Contains(trimmed, "<a ")) {
		contentType = "text/html; charset=\"utf-8\""
	}

	headers := map[string]string{
		"From":         s.From,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": contentType,
	}

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	auth := smtp.PlainAuth("", s.User, s.Pass, s.Host)

	// TLS handling:
	// - Port 465 usually expects implicit TLS.
	// - Port 587 usually expects STARTTLS (plaintext connect, then upgrade).
	if s.UseTLS {
		if s.Port == "465" {
			conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.Host})
			if err != nil {
				return err
			}
			c, err := smtp.NewClient(conn, s.Host)
			if err != nil {
				return err
			}
			defer c.Quit()
			if err := c.Auth(auth); err != nil {
				return err
			}
			if err := c.Mail(s.From); err != nil {
				return err
			}
			if err := c.Rcpt(to); err != nil {
				return err
			}
			w, err := c.Data()
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(msg.String()))
			if closeErr := w.Close(); err == nil {
				err = closeErr
			}
			return err
		}

		c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		defer c.Quit()
		if err := c.Hello("localhost"); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				return err
			}
		}
		if err := c.Auth(auth); err != nil {
			return err
		}
		if err := c.Mail(s.From); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(msg.String()))
		if closeErr := w.Close(); err == nil {
			err = closeErr
		}
		return err
	}

	return smtp.SendMail(addr, auth, s.From, []string{to}, []byte(msg.String()))
}
