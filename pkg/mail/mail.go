package mail

import (
	"fmt"
	"strconv"

	"gopkg.in/gomail.v2"
)

type MailerI interface {
	SendOTP(to, otp string) error
}

type GomailMailer struct {
	Dialer *gomail.Dialer
	From   string
}

func New(host, port, username, password, from string) MailerI {
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum <= 0 {
		portNum = 587
	}

	dialer := gomail.NewDialer(host, portNum, username, password)

	return &GomailMailer{
		Dialer: dialer,
		From:   from,
	}
}

func (m *GomailMailer) SendOTP(to, otp string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.From)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Your Verification Code")
	msg.SetBody("text/plain", fmt.Sprintf("Your OTP code is: %s\nIt expires in 5 minutes.", otp))

	return m.Dialer.DialAndSend(msg)
}
