package ports

import "context"

// GoogleIdentity son los datos verificados del idToken de Google Sign-In.
type GoogleIdentity struct {
	// Sub es el identificador inmutable de la cuenta Google ('sub' del idToken).
	// Es la LLAVE estable para reconciliar/vincular; el email se recicla y NO se
	// usa como llave.
	Sub    string
	Email  string
	Nombre string // "name" del idToken
	// EmailVerified refleja el claim email_verified: Google da fe de que el
	// dueno del idToken controla ese correo. Senal de propiedad del email.
	EmailVerified bool
}

// GoogleVerifier verifica el idToken de Google (firma, issuer, audience, exp)
// y extrae la identidad.
type GoogleVerifier interface {
	// Verify valida el idToken contra Google. Si la verificacion falla
	// (firma, iss, aud, exp): error (el caso de uso lo traduce a 401).
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}
