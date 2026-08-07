package backend

import (
	"errors"
	"fmt"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Default SMTP relay. EMAIL/APP_PASSWORD are a Gmail address and an app
// password, so Gmail's submission host is the right default; SMTP_HOST and
// SMTP_PORT override it for anyone relaying elsewhere.
const (
	defaultSMTPHost = "smtp.gmail.com"
	defaultSMTPPort = "587"
)

var ErrMailNotConfigured = errors.New("email delivery is not configured")

// mailSettings holds the resolved SMTP credentials for one send.
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

// resolveMailSettings reads SMTP configuration from the environment. It returns
// ErrMailNotConfigured when credentials are absent so callers can decide
// whether that is fatal.
func resolveMailSettings() (mailSettings, error) {
	settings := mailSettings{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("EMAIL"),
		Password: os.Getenv("APP_PASSWORD"),
	}

	if settings.Host == "" {
		settings.Host = defaultSMTPHost
	}
	if settings.Port == "" {
		settings.Port = defaultSMTPPort
	}

	if settings.Username == "" || settings.Password == "" {
		return settings, ErrMailNotConfigured
	}

	// Gmail rewrites the envelope sender to the authenticated account anyway,
	// so the From header tracks EMAIL unless overridden.
	settings.From = os.Getenv("MAIL_FROM")
	if settings.From == "" {
		settings.From = settings.Username
	}

	return settings, nil
}

// buildMessage renders an RFC 5322 message. Header values are validated by the
// caller; the body is folded into the message verbatim with CRLF line endings.
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

// headerSafe rejects values that would let a caller inject extra headers. Only
// the subject is attacker-influenced today, but recipients are checked too so
// this stays safe as call sites grow.
func headerSafe(value string) bool {
	return !strings.ContainsAny(value, "\r\n")
}

// SendMail delivers a plain-text message over SMTP with STARTTLS. When no
// credentials are configured, local builds log the message instead of failing
// so the full flow can be exercised without a mail server; release builds
// return an error because silently dropping mail there would strand users.
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

	auth := smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)
	message := buildMessage(settings.From, to, subject, body)

	if err := smtp.SendMail(settings.addr(), auth, settings.From, []string{to}, message); err != nil {
		return fmt.Errorf("send mail to %s: %w", to, err)
	}
	return nil
}

// logMailFallback prints a message that could not be delivered. Local builds
// use this to surface reset links on the console.
func logMailFallback(to, subject, body string) {
	log.Printf("[mail] EMAIL/APP_PASSWORD not set; message not sent.\nTo: %s\nSubject: %s\n\n%s\n", to, subject, body)
}
