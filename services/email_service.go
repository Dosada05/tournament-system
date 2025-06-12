package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/Dosada05/tournament-system/config"
)

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

func (s *EmailService) SendEmail(to []string, subject string, body string) error {
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)

	msg := []byte("To: " + to[0] + "\r\n" +
		"From: " + s.cfg.SMTPFrom + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n" +
		"\r\n" +
		body + "\r\n")

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         s.cfg.SMTPHost,
	}

	var client *smtp.Client
	if s.cfg.SMTPPort == 465 {
		// Прямое TLS-соединение (обычно порт 465)
		conn, err := tls.Dial("tcp", addr, tlsconfig)
		if err != nil {
			return fmt.Errorf("ошибка TLS соединения: %w", err)
		}
		defer conn.Close()
		client, err = smtp.NewClient(conn, s.cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("ошибка создания SMTP клиента: %w", err)
		}
	} else {
		// STARTTLS (обычно порт 587)
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("ошибка соединения SMTP: %w", err)
		}
		client = c
		if err = client.StartTLS(tlsconfig); err != nil {
			client.Close()
			return fmt.Errorf("ошибка команды STARTTLS: %w", err)
		}
	}
	defer client.Quit()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("ошибка аутентификации SMTP: %w", err)
	}

	if err := client.Mail(s.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("ошибка MAIL FROM: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("ошибка RCPT TO: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("ошибка команды DATA: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("ошибка записи сообщения: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("ошибка закрытия DATA: %w", err)
	}

	return nil
}

func (s *EmailService) GenerateEmailBody(templatePath string, data interface{}) (string, error) {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга шаблона %s: %w", templatePath, err)
	}

	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return "", fmt.Errorf("ошибка выполнения шаблона %s: %w", templatePath, err)
	}

	return body.String(), nil
}

func (s *EmailService) SendWelcomeEmail(userEmail string, confirmationToken string) error {
	subject := "Добро пожаловать в Tournament System!"
	templateData := struct {
		Email            string
		ConfirmationLink string
	}{
		Email: userEmail,
		// Используем ваш домен heartbit.live и предполагаем, что порт будет стандартным для HTTPS (443) или HTTP (80)
		// и будет обрабатываться прокси-сервером (например, Nginx), поэтому порт в ссылке не указываем явно.
		// Если ваше приложение напрямую слушает на s.cfg.ServerPort и доступно извне на этом порту,
		// то можно вернуть порт в ссылку: fmt.Sprintf("https://heartbit.live:%d/confirm-email?token=%s", s.cfg.ServerPort, confirmationToken)
		// или fmt.Sprintf("http://heartbit.live:%d/confirm-email?token=%s", s.cfg.ServerPort, confirmationToken)
		// В зависимости от того, используется ли HTTPS. Для продакшена всегда рекомендуется HTTPS.
		ConfirmationLink: fmt.Sprintf("%s/confirm-email?token=%s", s.cfg.PublicURL, confirmationToken),
	}

	htmlBody, err := s.GenerateEmailBody("templates/emails/welcome_email.html", templateData)
	if err != nil {
		return fmt.Errorf("ошибка генерации тела приветственного письма: %w", err)
	}

	return s.SendEmail([]string{userEmail}, subject, htmlBody)
}

func (s *EmailService) SendPasswordResetEmail(userEmail string, resetToken string) error {
	subject := "Сброс пароля для Heartbit"
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.PublicURL, resetToken)
	templateData := struct {
		Email     string
		ResetLink string
	}{
		Email:     userEmail,
		ResetLink: resetLink,
	}

	htmlBody, err := s.GenerateEmailBody("templates/emails/password_reset_email.html", templateData)
	if err != nil {
		return fmt.Errorf("ошибка генерации тела письма для сброса пароля: %w", err)
	}

	return s.SendEmail([]string{userEmail}, subject, htmlBody)
}

func (s *EmailService) SendTeamInviteEmail(userEmail, teamName, inviteLink string) error {
	subject := fmt.Sprintf("Приглашение в команду %s", teamName)
	data := struct {
		TeamName   string
		InviteLink string
	}{
		TeamName:   teamName,
		InviteLink: inviteLink,
	}
	htmlBody, err := s.GenerateEmailBody("templates/emails/team_invite_email.html", data)
	if err != nil {
		return fmt.Errorf("ошибка генерации тела письма-приглашения: %w", err)
	}
	return s.SendEmail([]string{userEmail}, subject, htmlBody)
}

func (s *EmailService) SendTournamentStatusEmail(userEmail, tournamentName, status, link string) error {
	subject := fmt.Sprintf("Турнир '%s': %s", tournamentName, status)
	data := struct {
		TournamentName string
		Status         string
		Link           string
	}{
		TournamentName: tournamentName,
		Status:         status,
		Link:           link,
	}
	htmlBody, err := s.GenerateEmailBody("templates/emails/tournament_status_email.html", data)
	if err != nil {
		return fmt.Errorf("ошибка генерации тела письма о статусе турнира: %w", err)
	}
	return s.SendEmail([]string{userEmail}, subject, htmlBody)
}

func (s *EmailService) SendSystemNotificationEmail(emails []string, subject, message string) error {
	for _, email := range emails {
		if err := s.SendEmail([]string{email}, subject, message); err != nil {
			return fmt.Errorf("ошибка отправки системного уведомления %s: %w", email, err)
		}
	}
	return nil
}
