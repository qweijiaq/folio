// Package mail 负责发送验证码邮件。
// 未配置 SMTP 时整个邮件功能会自动关闭，站点其余部分不受影响。
package mail

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"folio/server/config"
)

type Mailer interface {
	Enabled() bool
	Send(ctx context.Context, to, subject, body string) error
}

// New 根据配置返回可用的邮件发送器。
// FOLIO_MAIL_DEBUG=1 时只把邮件写进日志，并在接口响应里返回验证码，方便本地自测。
func New(cfg config.Config) Mailer {
	if cfg.MailDebug {
		return &debugMailer{from: firstNonEmpty(cfg.SMTPFrom, cfg.SMTPUser, "no-reply@folio.local")}
	}
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return disabledMailer{}
	}
	return &smtpMailer{
		Host:        strings.TrimSpace(cfg.SMTPHost),
		Port:        cfg.SMTPPort,
		User:        strings.TrimSpace(cfg.SMTPUser),
		Password:    cfg.SMTPPassword,
		From:        firstNonEmpty(cfg.SMTPFrom, cfg.SMTPUser),
		FromName:    firstNonEmpty(cfg.SMTPFromName, cfg.SiteName, "知微"),
		ImplicitTLS: cfg.SMTPTLS,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type disabledMailer struct{}

func (disabledMailer) Enabled() bool { return false }

func (disabledMailer) Send(context.Context, string, string, string) error {
	return fmt.Errorf("邮件服务未配置")
}

type debugMailer struct {
	from string
}

func (d *debugMailer) Enabled() bool { return true }

func (d *debugMailer) Send(_ context.Context, to, subject, body string) error {
	log.Printf("[mail:debug] from=%s to=%s subject=%s\n%s", d.from, to, subject, body)
	return nil
}

type smtpMailer struct {
	Host        string
	Port        int
	User        string
	Password    string
	From        string
	FromName    string
	ImplicitTLS bool
}

func (m *smtpMailer) Enabled() bool {
	return m.Host != "" && m.From != ""
}

func (m *smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	dialer := &net.Dialer{Timeout: 12 * time.Second}

	var conn net.Conn
	var err error
	if m.ImplicitTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: m.Host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接邮件服务器失败: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(25 * time.Second))

	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("初始化邮件连接失败: %w", err)
	}
	defer client.Close()

	if !m.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{
				ServerName: m.Host,
				MinVersion: tls.VersionTLS12,
			}); err != nil {
				return fmt.Errorf("STARTTLS 失败: %w", err)
			}
		}
	}
	if m.User != "" {
		if err := client.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return fmt.Errorf("邮件账号认证失败: %w", err)
		}
	}
	if err := client.Mail(m.From); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("开始写入邮件失败: %w", err)
	}
	if _, err := io.WriteString(writer, m.message(to, subject, body)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("写入邮件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	_ = client.Quit()
	return nil
}

func (m *smtpMailer) message(to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + formatAddress(m.FromName, m.From) + "\r\n")
	b.WriteString("To: " + formatAddress("", to) + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("\r\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		b.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded + "\r\n")
	return b.String()
}

func formatAddress(name, address string) string {
	if strings.TrimSpace(name) == "" {
		return address
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", name), address)
}
