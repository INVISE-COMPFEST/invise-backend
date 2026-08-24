package mail

import (
	"fmt"
	"net"
	"net/smtp"
)

type MailerI interface {
	SendOTP(to, otp string) error
}

type smtpMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func New(host, port, username, password, from string) MailerI {
	return &smtpMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (m *smtpMailer) SendOTP(to, otp string) error {
	addr := net.JoinHostPort(m.host, m.port)
	auth := smtp.PlainAuth("", m.username, m.password, m.host)

	subject := "Your Verification Code"
	body := fmt.Sprintf("Your OTP code is: %s\nIt expires in 5 minutes.", otp)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.from, to, subject, body)

	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}
