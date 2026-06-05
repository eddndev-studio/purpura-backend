package ports

import "context"

// GoogleIdentity son los datos verificados del idToken de Google Sign-In.
type GoogleIdentity struct {
	Email  string
	Nombre string // "name" del idToken
}

// GoogleVerifier verifica el idToken de Google (firma, issuer, audience, exp)
// y extrae la identidad.
type GoogleVerifier interface {
	// Verify valida el idToken contra Google. Si la verificacion falla
	// (firma, iss, aud, exp): error (el caso de uso lo traduce a 401).
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}
