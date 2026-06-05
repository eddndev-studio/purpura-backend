package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

func TestBcryptHasher_HashAndCompare(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost) // cost minimo: rapido en pruebas
	hash, err := h.Hash(context.Background(), "secretaPurpura1")
	if err != nil {
		t.Fatalf("Hash fallo: %v", err)
	}
	if hash == "secretaPurpura1" || hash == "" {
		t.Fatalf("el hash no debe ser la contrasena en claro")
	}
	if err := h.Compare(context.Background(), hash, "secretaPurpura1"); err != nil {
		t.Errorf("Compare correcto fallo: %v", err)
	}
	if err := h.Compare(context.Background(), hash, "incorrecta"); !errors.Is(err, domain.ErrInvalidCredential) {
		t.Errorf("Compare incorrecto -> ErrInvalidCredential, obtuve %v", err)
	}
}
