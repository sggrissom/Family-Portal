package backend

import (
	"crypto/tls"
	"errors"
	"family/cfg"
	"fmt"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"
)

const (
	defaultSMTPHost = "smtp.gmail.com"
	defaultSMTPPort = "587"
)

const (
	defaultRelayHost = "127.0.0.1"
	defaultRelayPort = "25"
)

const (
	mailDialTimeout = 10 * time.Second
	mailSendTimeout = 30 * time.Second
)

var ErrMailNotConfigured = errors.New("email delivery is not configured")

type mailSettings struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (s mailSettings) addr() string {
	return s.Host + ":" + s.Port
}

func (s mailSettings) useAuth() bool {
	return s.Username != "" && s.Password != ""
}

func resolveMailSettings() (mailSettings, error) {
	settings := mailSettings{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("EMAIL"),
		Password: os.Getenv("APP_PASSWORD"),
		From:     os.Getenv("MAIL_FROM"),
	}

	switch {
	case settings.From != "":
	case settings.useAuth():
		settings.From = settings.Username
	default:
		return settings, ErrMailNotConfigured
	}

	if settings.Host == "" {
		if settings.useAuth() {
			settings.Host = defaultSMTPHost
		} else {
			settings.Host = defaultRelayHost
		}
	}
	if settings.Port == "" {
		if settings.useAuth() {
			settings.Port = defaultSMTPPort
		} else {
			settings.Port = defaultRelayPort
		}
	}

	return settings, nil
}

func buildMessage(from, to, subject, body string) []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "From: %s\r\n", from)
	fmt.Fprintf(&sb, "To: %s\r\n", to)
	fmt.Fprintf(&sb, "Subject: %s\r\n", subject)
	fmt.Fprintf(&sb, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	return []byte(sb.String())
}

func headerSafe(value string) bool {
	return !strings.ContainsAny(value, "\r\n")
}

func SendMail(to, subject, body string) error {
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	if !headerSafe(subject) || !headerSafe(to) {
		return errors.New("mail headers must not contain line breaks")
	}

	settings, err := resolveMailSettings()
	if err != nil {
		return err
	}

	message := buildMessage(settings.From, to, subject, body)
	if err := sendSMTP(settings, to, message); err != nil {
		return fmt.Errorf("send mail to %s: %w", to, err)
	}
	return nil
}

func sendSMTP(settings mailSettings, to string, message []byte) error {
	conn, err := net.DialTimeout("tcp", settings.addr(), mailDialTimeout)
	if err != nil {
		return err
	}

	if err := conn.SetDeadline(time.Now().Add(mailSendTimeout)); err != nil {
		conn.Close()
		return err
	}

	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: settings.Host}); err != nil {
			return err
		}
	}

	if settings.useAuth() {
		auth := smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(settings.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(message); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func isPermanentMailError(err error) bool {
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		return protoErr.Code >= 500
	}
	return false
}

func logMailFallback(to, subject, body string) {
	if cfg.IsRelease {
		log.Printf("[mail] outbound mail is not configured; message not sent. To: %s, Subject: %s", redactEmail(to), subject)
		return
	}
	log.Printf("[mail] neither MAIL_FROM nor EMAIL/APP_PASSWORD set; message not sent.\nTo: %s\nSubject: %s\n\n%s\n", to, subject, body)
}

type MailJob struct {
	To      string
	Subject string
	Body    string
	Kind    string
}

func deliverNow(job MailJob) error {
	err := SendMail(job.To, job.Subject, job.Body)
	if errors.Is(err, ErrMailNotConfigured) && !cfg.IsRelease {
		logMailFallback(job.To, job.Subject, job.Body)
		return nil
	}
	return err
}
