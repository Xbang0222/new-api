package common

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type EmailAttachment struct {
	FileName    string
	ContentType string
	Data        []byte
}

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	return AutoSMTPAuth(SMTPAccount, SMTPToken)
}

func shouldAuthenticateSMTP() bool {
	return SMTPAccount != "" && SMTPToken != ""
}

func smtpTLSConfig() *tls.Config {
	return &tls.Config{
		ServerName:         SMTPServer,
		InsecureSkipVerify: SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

func newSMTPClient(addr string) (*smtp.Client, error) {
	if SMTPSSLEnabled || (SMTPPort == 465 && !SMTPStartTLSEnabled) {
		conn, err := tls.Dial("tcp", addr, smtpTLSConfig())
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, err
	}

	if SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

func SendEmail(subject string, receiver string, content string) error {
	return SendEmailWithAttachments(subject, receiver, content, nil)
}

func SendEmailWithAttachments(subject string, receiver string, content string, attachments []EmailAttachment) error {
	OptionMapRWMutex.RLock()
	provider := OptionMap["EmailProvider"]
	resendKey := OptionMap["ResendApiKey"]
	resendFrom := OptionMap["ResendFrom"]
	OptionMapRWMutex.RUnlock()
	// One selector controls delivery. Never fall back to a second provider:
	// a failed response does not prove that the first provider did not send.
	switch provider {
	case "resend":
		return sendResendEmail(resendKey, resendFrom, subject, receiver, content, attachments...)
	case "", "smtp": // Preserve existing installations without a selector.
	default:
		return NewMessage("Email sending method must be SMTP or Resend")
	}
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	messageID, err := generateMessageID()
	if err != nil {
		return err
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return NewMessage("SMTP server is not configured")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	message, err := buildEmailMessage(receiver, encodedSubject, messageID, content, attachments)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	client, err := newSMTPClient(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if shouldAuthenticateSMTP() {
		if err = client.Auth(getSMTPAuth()); err != nil {
			return err
		}
	}
	if err = client.Mail(SMTPFrom); err != nil {
		return err
	}
	for _, recipient := range strings.Split(receiver, ";") {
		if err = client.Rcpt(strings.TrimSpace(recipient)); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(message); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = client.Quit(); err != nil {
		SysError(LogText("failed to send email to %s: %v", receiver, err))
	}
	return err
}

func buildEmailMessage(receiver string, encodedSubject string, messageID string, content string, attachments []EmailAttachment) ([]byte, error) {
	var message bytes.Buffer
	_, _ = fmt.Fprintf(&message, "To: %s\r\n", receiver)
	_, _ = fmt.Fprintf(&message, "From: %s <%s>\r\n", SystemName, SMTPFrom)
	_, _ = fmt.Fprintf(&message, "Subject: %s\r\n", encodedSubject)
	_, _ = fmt.Fprintf(&message, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	_, _ = message.WriteString("MIME-Version: 1.0\r\n")

	if len(attachments) == 0 {
		_, _ = message.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		_, _ = message.WriteString(content)
		_, _ = message.WriteString("\r\n")
		return message.Bytes(), nil
	}

	multipartWriter := multipart.NewWriter(&message)
	_, _ = fmt.Fprintf(&message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", multipartWriter.Boundary())
	bodyHeader := textproto.MIMEHeader{}
	bodyHeader.Set("Content-Type", "text/html; charset=UTF-8")
	bodyPart, err := multipartWriter.CreatePart(bodyHeader)
	if err != nil {
		return nil, err
	}
	if _, err = bodyPart.Write([]byte(content)); err != nil {
		return nil, err
	}

	for _, attachment := range attachments {
		fileName := filepath.Base(strings.TrimSpace(attachment.FileName))
		fileName = strings.NewReplacer("\r", "", "\n", "").Replace(fileName)
		if fileName == "" || fileName == "." {
			fileName = "attachment"
		}
		contentType := strings.TrimSpace(attachment.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", mime.FormatMediaType(contentType, map[string]string{"name": fileName}))
		header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
		header.Set("Content-Transfer-Encoding", "base64")
		part, createErr := multipartWriter.CreatePart(header)
		if createErr != nil {
			return nil, createErr
		}
		encoder := base64.NewEncoder(base64.StdEncoding, part)
		if _, err = encoder.Write(attachment.Data); err != nil {
			_ = encoder.Close()
			return nil, err
		}
		if err = encoder.Close(); err != nil {
			return nil, err
		}
	}

	if err = multipartWriter.Close(); err != nil {
		return nil, err
	}
	return message.Bytes(), nil
}
