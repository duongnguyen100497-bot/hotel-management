package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Message represents an outbound email to be delivered over SMTP.
type Message struct {
	To        string
	Subject   string
	TextBody  string
	HTMLBody  string
	MessageID string
}

// Sender defines the outbound email transport contract.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Config configures the SMTP client for local Mailpit (plain SMTP) or production relays (STARTTLS + PLAIN Auth).
type Config struct {
	Host        string
	Port        int
	Username    string
	Password    string
	From        string
	DialTimeout time.Duration
}

// Client implements Sender over net/smtp with automatic STARTTLS and AUTH support when configured.
type Client struct {
	cfg Config
}

// NewClient constructs an SMTP Client.
func NewClient(cfg Config) *Client {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port <= 0 {
		cfg.Port = 1025
	}
	if cfg.From == "" {
		cfg.From = "no-reply@hotel.local"
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	return &Client{cfg: cfg}
}

// Send delivers msg via the configured SMTP server.
func (c *Client) Send(ctx context.Context, msg Message) error {
	toAddr, err := sanitizeEmailAddress(msg.To)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	fromAddr, err := sanitizeEmailAddress(c.cfg.From)
	if err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}

	rawMail := buildMIMEMessage(fromAddr, toAddr, msg)
	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))

	dialer := net.Dialer{Timeout: c.cfg.DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp %s: %w", addr, err)
	}
	defer conn.Close()

	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(c.cfg.DialTimeout))
	}

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{
			ServerName: c.cfg.Host,
			MinVersion: tls.VersionTLS12,
		}
		if strings.EqualFold(c.cfg.Host, "localhost") || c.cfg.Host == "127.0.0.1" {
			tlsCfg.InsecureSkipVerify = true
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			// Local Mailpit without TLS cert may advertise or not advertise STARTTLS; only fail if auth is configured.
			if c.cfg.Username != "" {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if c.cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(fromAddr); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(toAddr); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(rawMail); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

func sanitizeEmailAddress(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", fmt.Errorf("header injection attempt detected")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", err
	}
	return parsed.Address, nil
}

func buildMIMEMessage(fromAddr, toAddr string, msg Message) []byte {
	const boundary = "hotel-mgmt-mime-boundary-2026"
	subject := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(msg.Subject), "\r", ""), "\n", " ")
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)

	var b strings.Builder
	b.WriteString("From: " + fromAddr + "\r\n")
	b.WriteString("To: " + toAddr + "\r\n")
	b.WriteString("Subject: " + encodedSubject + "\r\n")
	if msg.MessageID != "" {
		cleanID := strings.ReplaceAll(strings.ReplaceAll(msg.MessageID, "\r", ""), "\n", "")
		b.WriteString("Message-ID: <" + cleanID + "@hotel.local>\r\n")
	}
	b.WriteString("MIME-Version: 1.0\r\n")

	if strings.TrimSpace(msg.HTMLBody) == "" {
		b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(msg.TextBody)
		b.WriteString("\r\n")
		return []byte(b.String())
	}

	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.TextBody)
	b.WriteString("\r\n--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.HTMLBody)
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return []byte(b.String())
}
