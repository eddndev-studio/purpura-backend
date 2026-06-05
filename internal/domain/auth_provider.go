package domain

// AuthProvider es el proveedor de autenticacion con el que se creo la cuenta.
type AuthProvider string

const (
	AuthGoogle   AuthProvider = "google"
	AuthPassword AuthProvider = "password"
)

var validAuthProviders = map[AuthProvider]bool{
	AuthGoogle:   true,
	AuthPassword: true,
}

// ParseAuthProvider valida y normaliza un proveedor de autenticacion.
func ParseAuthProvider(s string) (AuthProvider, error) {
	p := AuthProvider(s)
	if !validAuthProviders[p] {
		return "", ErrInvalidAuthProvider
	}
	return p, nil
}

// Valid indica si el proveedor es uno de los permitidos.
func (p AuthProvider) Valid() bool {
	return validAuthProviders[p]
}
