package ports

import (
	"context"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// IssuedToken es el resultado de emitir un access token. El adaptador HTTP lo
// serializa como { accessToken, tokenType: "Bearer", expiresIn }.
type IssuedToken struct {
	AccessToken string
	TokenType   string // "Bearer"
	ExpiresIn   int64  // segundos hasta exp (p.ej. 86400)
}

// Claims son los claims verificados que el middleware inyecta en el contexto.
type Claims struct {
	Subject      string // User.id (identidad autoritativa)
	Email        string
	AuthProvider string
	JTI          string
}

// TokenService emite y verifica el JWT propio de Purpura.
type TokenService interface {
	// Issue emite un access token para el usuario dado.
	Issue(ctx context.Context, u *domain.User) (IssuedToken, error)

	// Verify valida firma, iss, aud y expiracion. Token invalido o expirado:
	// error (el middleware lo traduce a 401 unauthorized). No consulta la BD.
	Verify(ctx context.Context, accessToken string) (Claims, error)
}
