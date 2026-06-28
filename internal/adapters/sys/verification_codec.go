package sys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// VerificationTokenCodec genera tokens opacos de verificacion de correo (32 bytes
// de crypto/rand, base64url sin padding) y los hashea con SHA-256 para
// almacenarlos. Es la unica pieza que conoce la criptografia; el caso de uso solo
// orquesta. Sin estado: seguro para uso concurrente.
type VerificationTokenCodec struct{}

var _ ports.VerificationTokenCodec = VerificationTokenCodec{}

// NewVerificationTokenCodec construye el codec.
func NewVerificationTokenCodec() VerificationTokenCodec { return VerificationTokenCodec{} }

// Mint genera un token aleatorio y devuelve el crudo (va en el correo) y su hash
// (lo unico que se persiste). 32 bytes = 256 bits de entropia, no adivinable.
func (VerificationTokenCodec) Mint() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("sys: generar token de verificacion: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

// Hash deriva el hash de un token presentado, para buscarlo en la BD.
func (VerificationTokenCodec) Hash(raw string) string { return hashToken(raw) }

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
