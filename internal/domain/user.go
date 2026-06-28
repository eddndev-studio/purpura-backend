package domain

import (
	"net/mail"
	"strings"
	"time"
)

// User es el propietario de los eventos: la raiz de identidad del dominio.
// Cada Event pertenece a exactamente un User.
type User struct {
	ID           string
	Email        string
	Nombre       string
	AuthProvider AuthProvider
	// GoogleSub es el 'sub' inmutable del idToken de Google cuando la cuenta
	// tiene Google adjunto (nil si no). Es la LLAVE de vinculacion: ortogonal a
	// AuthProvider (el proveedor de ORIGEN), permite que una cuenta password
	// entre tambien por Google una vez vinculada. Nunca se llavea por email.
	GoogleSub *string
	// EmailVerified indica si el dueno comprobo el correo (gate SUAVE: nunca
	// bloquea el login; la app lo usa solo para un aviso). Las cuentas de origen
	// Google nacen verificadas (el idToken da fe del correo).
	EmailVerified bool
	CreatedAt     time.Time
}

// GoogleLinked indica si la cuenta tiene Google adjunto (entra tambien por
// Google). Es el flag derivado que viaja en el payload del usuario.
func (u User) GoogleLinked() bool {
	return u.GoogleSub != nil
}

// NewUser construye un User validando las invariantes de dominio: email con
// formato valido, nombre no vacio y proveedor de autenticacion permitido.
// Normaliza el email a minusculas y recorta espacios de email y nombre.
// ID y CreatedAt los asigna el backend (capa de datos), no este constructor.
func NewUser(email, nombre string, provider AuthProvider) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return nil, ErrInvalidEmail
	}
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return nil, ErrEmptyName
	}
	if !provider.Valid() {
		return nil, ErrInvalidAuthProvider
	}
	return &User{
		Email:        email,
		Nombre:       nombre,
		AuthProvider: provider,
	}, nil
}

// validEmail acepta una direccion de correo simple (sin nombre para mostrar).
func validEmail(s string) bool {
	if s == "" {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	// Rechaza formas con nombre para mostrar ("Ana <ana@x.com>"): exigimos que
	// la direccion parseada sea identica a la entrada ya normalizada.
	return addr.Address == s
}
