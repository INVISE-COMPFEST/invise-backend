package mail_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"invise-backend/pkg/mail"
)

func TestNew(t *testing.T) {
	t.Run("creates gomailMailer with valid port", func(t *testing.T) {
		mailer := mail.New("smtp.example.com", "587", "user", "pass", "sender@example.com")
		assert.NotNil(t, mailer)

		gm, ok := mailer.(*mail.GomailMailer)
		assert.True(t, ok)
		assert.Equal(t, "sender@example.com", gm.From)
		assert.Equal(t, "smtp.example.com", gm.Dialer.Host)
		assert.Equal(t, 587, gm.Dialer.Port)
		assert.Equal(t, "user", gm.Dialer.Username)
		assert.Equal(t, "pass", gm.Dialer.Password)
	})

	t.Run("falls back to port 587 when port is invalid", func(t *testing.T) {
		mailer := mail.New("smtp.example.com", "invalid", "user", "pass", "Invise <sender@example.com>")
		assert.NotNil(t, mailer)

		gm, ok := mailer.(*mail.GomailMailer)
		assert.True(t, ok)
		assert.Equal(t, "Invise <sender@example.com>", gm.From)
		assert.Equal(t, 587, gm.Dialer.Port)
	})
}
