package backend

import (
	"bytes"
	"errors"
	"family/cfg"
	"log"
	"os"
	"strings"
	"testing"
)

func TestResolveMailSettings(t *testing.T) {
	t.Run("defaults to the Gmail submission host", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("MAIL_FROM", "")
		t.Setenv("EMAIL", "portal@example.com")
		t.Setenv("APP_PASSWORD", "app-password")

		settings, err := resolveMailSettings()
		if err != nil {
			t.Fatalf("resolveMailSettings() error = %v", err)
		}
		if settings.addr() != "smtp.gmail.com:587" {
			t.Errorf("addr() = %q, want smtp.gmail.com:587", settings.addr())
		}
		if settings.From != "portal@example.com" {
			t.Errorf("From = %q, want the EMAIL value", settings.From)
		}
	})

	t.Run("honours overrides", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_PORT", "2525")
		t.Setenv("MAIL_FROM", "noreply@example.com")
		t.Setenv("EMAIL", "portal@example.com")
		t.Setenv("APP_PASSWORD", "app-password")

		settings, err := resolveMailSettings()
		if err != nil {
			t.Fatalf("resolveMailSettings() error = %v", err)
		}
		if settings.addr() != "smtp.example.com:2525" {
			t.Errorf("addr() = %q", settings.addr())
		}
		if settings.From != "noreply@example.com" {
			t.Errorf("From = %q, want the MAIL_FROM value", settings.From)
		}
	})

	t.Run("relays through the local server without credentials", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("MAIL_FROM", "noreply@familyrecord.app")
		t.Setenv("EMAIL", "")
		t.Setenv("APP_PASSWORD", "")

		settings, err := resolveMailSettings()
		if err != nil {
			t.Fatalf("resolveMailSettings() error = %v", err)
		}
		if settings.addr() != "127.0.0.1:25" {
			t.Errorf("addr() = %q, want 127.0.0.1:25", settings.addr())
		}
		if settings.From != "noreply@familyrecord.app" {
			t.Errorf("From = %q, want the MAIL_FROM value", settings.From)
		}
		if settings.useAuth() {
			t.Error("useAuth() = true, want false when no credentials are set")
		}
	})

	t.Run("still authenticates when credentials accompany MAIL_FROM", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("MAIL_FROM", "noreply@example.com")
		t.Setenv("EMAIL", "portal@example.com")
		t.Setenv("APP_PASSWORD", "app-password")

		settings, err := resolveMailSettings()
		if err != nil {
			t.Fatalf("resolveMailSettings() error = %v", err)
		}
		if !settings.useAuth() {
			t.Error("useAuth() = false, want true when credentials are set")
		}
		if settings.addr() != "smtp.gmail.com:587" {
			t.Errorf("addr() = %q, want the submission host when credentials exist", settings.addr())
		}
	})

	t.Run("reports missing credentials", func(t *testing.T) {
		t.Setenv("MAIL_FROM", "")
		t.Setenv("EMAIL", "portal@example.com")
		t.Setenv("APP_PASSWORD", "")

		if _, err := resolveMailSettings(); !errors.Is(err, ErrMailNotConfigured) {
			t.Errorf("error = %v, want ErrMailNotConfigured", err)
		}
	})
}

func TestSendMailRejectsHeaderInjection(t *testing.T) {
	t.Setenv("EMAIL", "portal@example.com")
	t.Setenv("APP_PASSWORD", "app-password")

	tests := []struct {
		name    string
		to      string
		subject string
	}{
		{"newline in recipient", "victim@example.com\nBcc: attacker@example.com", "Hello"},
		{"newline in subject", "victim@example.com", "Hello\r\nBcc: attacker@example.com"},
		{"malformed recipient", "not-an-address", "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SendMail(tt.to, tt.subject, "body"); err == nil {
				t.Error("SendMail() accepted a message that could forge headers")
			}
		})
	}
}

func TestBuildMessageUsesCRLF(t *testing.T) {
	message := string(buildMessage("from@example.com", "to@example.com", "Subject line", "line one\nline two"))

	if !strings.Contains(message, "From: from@example.com\r\n") {
		t.Error("From header is missing or not CRLF terminated")
	}
	if !strings.Contains(message, "\r\n\r\nline one\r\nline two") {
		t.Error("body is not separated from headers with CRLF line endings")
	}
	if strings.Contains(strings.ReplaceAll(message, "\r\n", ""), "\n") {
		t.Error("message contains bare newlines")
	}
}

func TestLogMailFallbackKeepsBodiesOutOfReleaseLogs(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	const link = "https://familyrecord.app/reset-password?token=secrettokenvalue"
	logMailFallback("member@example.com", "Reset your password", "Open this link:\n"+link)

	output := buf.String()
	if cfg.IsRelease {
		if strings.Contains(output, link) {
			t.Error("release build logged the message body")
		}
		if strings.Contains(output, "member@example.com") {
			t.Error("release build logged the full recipient address")
		}
		return
	}

	if !strings.Contains(output, link) {
		t.Error("local build did not print the reset link")
	}
}
