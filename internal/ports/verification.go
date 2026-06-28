package ports

import (
	"context"
	"time"
)

// VerificationToken es un token de verificacion de correo persistido. Solo se
// guarda el hash (TokenHash); el valor crudo nunca toca la BD. UsedAt nil = sin
// usar.
type VerificationToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// VerificationTokenRepository persiste y consulta tokens de verificacion.
type VerificationTokenRepository interface {
	// Create persiste un token nuevo (ya con ID, TokenHash, ExpiresAt y CreatedAt
	// asignados por el caso de uso).
	Create(ctx context.Context, t *VerificationToken) error

	// FindByHash busca por el hash del token. Si no existe:
	// domain.ErrInvalidVerificationToken.
	FindByHash(ctx context.Context, hash string) (*VerificationToken, error)

	// MarkUsed marca el token como usado de forma atomica y de un solo uso.
	// Devuelve true si lo marco; false si ya estaba usado (carrera) o no existe.
	MarkUsed(ctx context.Context, id string, usedAt time.Time) (bool, error)
}

// VerificationTokenCodec genera y deriva el hash de los tokens opacos de
// verificacion. Se inyecta para que el caso de uso no haga criptografia
// directamente y para poder fijar valores en pruebas.
type VerificationTokenCodec interface {
	// Mint genera un token aleatorio: devuelve el valor crudo (va en el correo) y
	// su hash (lo unico que se persiste).
	Mint() (raw string, hash string, err error)
	// Hash deriva el hash de un token presentado, para buscarlo en la BD.
	Hash(raw string) string
}
