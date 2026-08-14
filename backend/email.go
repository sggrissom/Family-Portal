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

// Default SMTP relay. EMAIL/APP_PASSWORD are a Gmail address and an app
// password, so Gmail's submission host is the right default when they are
// present; SMTP_HOST and SMTP_PORT override it for anyone relaying elsewhere.
const (
	defaultSMTPHost = "smtp.gmail.com"
	defaultSMTPPort = "587"
)

// Default relay when no credentials are supplied. tiny-server-helper runs a
// Postfix instance bound to loopback that accepts unauthenticated submission
// from local apps and signs outbound mail with the sending domain's DKIM key,
// so no host, port, or secret needs to live in this repo. On that host the
// SMTP_HOST/SMTP_PORT below arrive from a systemd drop-in anyway.
const (
	defaultRelayHost = "127.0.0.1"
	defaultRelayPort = "25"
)

// Bounds on one delivery attempt. net/smtp imposes no deadline of its own, so
// without these a relay that accepts a connection and then stops responding
// would block the sender indefinitely — and since sends are queued, one such
// peer would stall every message behind it.
const (
	mailDialTimeout = 10 * time.Second
	mailSendTimeout = 30 * time.Second
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

// useAuth reports whether credentials were supplied. A local relay accepts
// unauthenticated submission, so their absence is a valid configuration rather
// than an error.
func (s mailSettings) useAuth() bool {
	return s.Username != "" && s.Password != ""
}

// resolveMailSettings reads SMTP configuration from the environment. Two
// arrangements are supported:
//
//   - MAIL_FROM set: mail is relayed as that address. Credentials are optional,
//     which is what lets the app hand messages to a loopback SMTP server that
//     authenticates nothing and signs everything.
//   - EMAIL and APP_PASSWORD set: mail is authenticated to a submission host
//     and sent as EMAIL.
//
// It returns ErrMailNotConfigured when neither applies, so callers can decide
// whether that is fatal.
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
		// Relayed as MAIL_FROM, with or without credentials.
	case settings.useAuth():
		// Gmail rewrites the envelope sender to the authenticated account
		// anyway, so the From header tracks EMAIL when MAIL_FROM is not set.
		settings.From = settings.Username
	default:
		return settings, ErrMailNotConfigured
	}

	// Credentials imply a submission host; their absence implies a local relay.
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

// SendMail delivers a plain-text message over SMTP, upgrading to STARTTLS when
// the server offers it and authenticating only when credentials are configured.
// Against a loopback relay there is typically nothing to authenticate to and
// nothing to protect on that hop.
//
// This is the synchronous path. Callers serving a user request should use
// QueueMail instead so a slow relay cannot hold the response open.
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

// sendSMTP runs the SMTP conversation for one message. It follows the same
// sequence smtp.SendMail does — opportunistic STARTTLS whenever the server
// advertises it, AUTH only when credentials exist — but drives the client
// directly so the connection can carry a deadline.
func sendSMTP(settings mailSettings, to string, message []byte) error {
	conn, err := net.DialTimeout("tcp", settings.addr(), mailDialTimeout)
	if err != nil {
		return err
	}

	// One deadline covers the whole exchange rather than each round trip, so a
	// peer cannot extend the send indefinitely by answering slowly but steadily.
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

	// PlainAuth refuses to run over an unencrypted connection, so it is built
	// only when credentials exist; a loopback relay authenticates nothing.
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

// isPermanentMailError reports whether the server refused the message in a way
// that retrying cannot fix. SMTP splits failures at the status code: 5xx is a
// verdict about this message (no such mailbox, rejected content), 4xx is a
// statement about right now (greylisting, over quota, out of disk).
func isPermanentMailError(err error) bool {
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		return protoErr.Code >= 500
	}
	return false
}

// logMailFallback reports a message that could not be delivered.
//
// The body is printed only in local builds, where seeing the reset link on the
// console is the point. A release build must never print it: these messages
// carry single-use password reset links, and a log file is a place they would
// sit readable long after the link was meant to expire.
func logMailFallback(to, subject, body string) {
	if cfg.IsRelease {
		log.Printf("[mail] outbound mail is not configured; message not sent. To: %s, Subject: %s", redactEmail(to), subject)
		return
	}
	log.Printf("[mail] neither MAIL_FROM nor EMAIL/APP_PASSWORD set; message not sent.\nTo: %s\nSubject: %s\n\n%s\n", to, subject, body)
}

// MailJob is one outbound message. Kind is a short label used in logs so a
// delivery failure can be traced back to the flow that produced it without
// putting the recipient or body in the log line.
type MailJob struct {
	To      string
	Subject string
	Body    string
	Kind    string
}

// deliverNow sends a job synchronously. When mail is not configured at all,
// local builds log the message instead of failing so flows that depend on a
// link — password reset above all — stay testable without a mail server;
// release builds report the error, because silently dropping mail there would
// strand users outside their accounts.
func deliverNow(job MailJob) error {
	err := SendMail(job.To, job.Subject, job.Body)
	if errors.Is(err, ErrMailNotConfigured) && !cfg.IsRelease {
		logMailFallback(job.To, job.Subject, job.Body)
		return nil
	}
	return err
}
