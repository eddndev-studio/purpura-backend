// Package auth agrupa los adaptadores driven de seguridad: emision/verificacion
// del JWT propio (TokenService), verificacion del idToken de Google
// (GoogleVerifier) y hashing de contrasenas (PasswordHasher). Implementan los
// puertos de internal/ports y se inyectan en el composition root (cmd/api).
package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// BcryptHasher implementa ports.PasswordHasher con bcrypt (cost configurable).
type BcryptHasher struct {
	cost int
}

var _ ports.PasswordHasher = BcryptHasher{}

// NewBcryptHasher construye el hasher. cost <= 0 usa bcrypt.DefaultCost; un cost
// fuera del rango aceptado por bcrypt se acota en GenerateFromPassword.
func NewBcryptHasher(cost int) BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return BcryptHasher{cost: cost}
}

// Hash deriva el hash bcrypt de la contrasena en claro.
func (h BcryptHasher) Hash(_ context.Context, plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare verifica la contrasena contra su hash. Desajuste -> ErrInvalidCredential
// (lo que Login colapsa a 401 sin filtrar existencia); otros fallos -> infra.
func (h BcryptHasher) Compare(_ context.Context, hash, plain string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return domain.ErrInvalidCredential
	}
	return err
}
