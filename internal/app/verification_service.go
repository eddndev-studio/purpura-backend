package app

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// VerificationService orquesta la verificacion de correo. El gate es SUAVE: nunca
// bloquea el login; solo cambia el flag email_verified que la app usa para un
// aviso. La seguridad del token la dan: 256 bits de entropia, solo se guarda el
// hash, expiracion y un solo uso atomico.
type VerificationService struct {
	Users  ports.UserRepository
	Tokens ports.VerificationTokenRepository
	Codec  ports.VerificationTokenCodec
	Email  ports.EmailSender
	Clock  ports.Clock
	IDs    ports.IDGenerator

	// TTL es la validez del token; VerifyURL es la base del enlace del correo
	// (p.ej. https://purpura.eddn.dev/verify), al que se le anade ?token=<crudo>.
	TTL       time.Duration
	VerifyURL string
}

// RequestVerification crea un token de verificacion para el usuario autenticado y
// le envia el correo. Idempotente: si el correo ya esta verificado, no hace nada
// (no envia otro correo). userID viene del sub del JWT.
func (s *VerificationService) RequestVerification(ctx context.Context, userID string) error {
	u, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.EmailVerified {
		return nil // ya verificado: nada que hacer
	}
	raw, hash, err := s.Codec.Mint()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	tok := &ports.VerificationToken{
		ID:        s.IDs.NewID(),
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(s.TTL),
		CreatedAt: now,
	}
	if err := s.Tokens.Create(ctx, tok); err != nil {
		return err
	}
	return s.Email.SendVerificationEmail(ctx, ports.VerificationEmail{
		To:        u.Email,
		Nombre:    u.Nombre,
		VerifyURL: s.buildVerifyURL(raw),
	})
}

// ConfirmVerification valida el token crudo presentado y marca el correo como
// verificado. Token inexistente o ya usado -> ErrInvalidVerificationToken;
// expirado -> ErrVerificationTokenExpired. El uso es atomico (MarkUsed), de modo
// que dos confirmaciones concurrentes solo una tiene efecto.
func (s *VerificationService) ConfirmVerification(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return domain.ErrInvalidVerificationToken
	}
	tok, err := s.Tokens.FindByHash(ctx, s.Codec.Hash(rawToken))
	if err != nil {
		return err // FindByHash devuelve ErrInvalidVerificationToken si no existe
	}
	if tok.UsedAt != nil {
		return domain.ErrInvalidVerificationToken // ya usado
	}
	now := s.Clock.Now()
	if !now.Before(tok.ExpiresAt) {
		return domain.ErrVerificationTokenExpired
	}
	marked, err := s.Tokens.MarkUsed(ctx, tok.ID, now)
	if err != nil {
		return err
	}
	if !marked {
		// Carrera: otra confirmacion lo marco entre el find y el mark.
		return domain.ErrInvalidVerificationToken
	}
	return s.Users.SetEmailVerified(ctx, tok.UserID)
}

// buildVerifyURL adjunta ?token=<crudo> a la base, respetando si ya trae query.
func (s *VerificationService) buildVerifyURL(raw string) string {
	sep := "?"
	if strings.Contains(s.VerifyURL, "?") {
		sep = "&"
	}
	return s.VerifyURL + sep + "token=" + url.QueryEscape(raw)
}
