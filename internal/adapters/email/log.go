// Package email implementa ports.EmailSender. En produccion usa Resend; cuando no
// hay RESEND_API_KEY, LogSender registra el enlace en el log para no bloquear el
// despliegue (el flujo funciona end-to-end en local/staging sin proveedor real).
package email

import (
	"context"
	"log/slog"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// LogSender registra el enlace de verificacion en vez de enviarlo. Fallback sin
// RESEND_API_KEY.
type LogSender struct {
	logger *slog.Logger
}

var _ ports.EmailSender = (*LogSender)(nil)

// NewLogSender construye el sender de log.
func NewLogSender(logger *slog.Logger) *LogSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogSender{logger: logger}
}

// SendVerificationEmail registra el destinatario y el enlace. Nunca falla.
func (s *LogSender) SendVerificationEmail(_ context.Context, msg ports.VerificationEmail) error {
	s.logger.Info("correo de verificacion (modo log, sin envio real)",
		"to", msg.To, "verifyURL", msg.VerifyURL)
	return nil
}
