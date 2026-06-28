package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// issuers de Google aceptados para el idToken (04 seccion 3.1).
var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

// GoogleVerifier implementa ports.GoogleVerifier validando el idToken de Google
// Sign-In: firma contra las llaves publicas de Google (cacheadas por idtoken),
// audience == clientID y expiracion, mas la comprobacion de issuer. Extrae email
// y nombre.
type GoogleVerifier struct {
	clientID string
	// validate se inyecta para poder probar la extraccion de claims sin red;
	// en produccion es idtoken.Validate (descarga/cachea las llaves de Google).
	validate func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)
}

var _ ports.GoogleVerifier = (*GoogleVerifier)(nil)

// NewGoogleVerifier construye el verificador para el client id de Purpura.
func NewGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{
		clientID: clientID,
		validate: idtoken.Validate,
	}
}

// Verify valida el idToken y devuelve la identidad. Cualquier fallo (firma, aud,
// exp, issuer, email ausente) -> error, que el caso de uso traduce a 401.
func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (ports.GoogleIdentity, error) {
	payload, err := v.validate(ctx, idToken, v.clientID)
	if err != nil {
		return ports.GoogleIdentity{}, fmt.Errorf("google: idToken no valido: %w", err)
	}
	if !googleIssuers[payload.Issuer] {
		return ports.GoogleIdentity{}, fmt.Errorf("google: issuer no confiable: %q", payload.Issuer)
	}
	// sub: identificador inmutable de la cuenta (llave de vinculacion). Sin el no
	// se puede reconciliar de forma estable -> rechazar.
	if payload.Subject == "" {
		return ports.GoogleIdentity{}, fmt.Errorf("google: idToken sin sub")
	}
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return ports.GoogleIdentity{}, fmt.Errorf("google: idToken sin email")
	}
	name, _ := payload.Claims["name"].(string)
	return ports.GoogleIdentity{
		Sub:           payload.Subject,
		Email:         email,
		Nombre:        name,
		EmailVerified: claimBool(payload.Claims["email_verified"]),
	}, nil
}

// claimBool interpreta un claim como booleano: acepta el bool nativo o la cadena
// "true" (algunos flujos serializan email_verified como string). Cualquier otra
// cosa (incluida la ausencia) es false.
func claimBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	default:
		return false
	}
}
