package domain

import (
	"errors"
	"testing"
)

func TestParseAuthProvider(t *testing.T) {
	for _, s := range []string{"google", "password"} {
		p, err := ParseAuthProvider(s)
		if err != nil {
			t.Errorf("ParseAuthProvider(%q) error inesperado: %v", s, err)
		}
		if string(p) != s {
			t.Errorf("ParseAuthProvider(%q) = %q", s, p)
		}
	}
	if _, err := ParseAuthProvider("github"); !errors.Is(err, ErrInvalidAuthProvider) {
		t.Errorf("proveedor invalido: err = %v, se esperaba ErrInvalidAuthProvider", err)
	}
}

func TestNewUserNormalizesEmailAndName(t *testing.T) {
	u, err := NewUser("  Alex@Example.COM ", "  Alejandro  ", AuthPassword)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if u.Email != "alex@example.com" {
		t.Errorf("email = %q, se esperaba normalizado a minusculas y sin espacios", u.Email)
	}
	if u.Nombre != "Alejandro" {
		t.Errorf("nombre = %q, se esperaba recortado", u.Nombre)
	}
	if u.AuthProvider != AuthPassword {
		t.Errorf("auth_provider = %q, se esperaba password", u.AuthProvider)
	}
}

func TestNewUserRejectsInvalidInput(t *testing.T) {
	if _, err := NewUser("no-es-correo", "Ana", AuthGoogle); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("email invalido: err = %v, se esperaba ErrInvalidEmail", err)
	}
	if _, err := NewUser("", "Ana", AuthGoogle); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("email vacio: err = %v, se esperaba ErrInvalidEmail", err)
	}
	if _, err := NewUser("ana@example.com", "   ", AuthGoogle); !errors.Is(err, ErrEmptyName) {
		t.Errorf("nombre vacio: err = %v, se esperaba ErrEmptyName", err)
	}
	if _, err := NewUser("ana@example.com", "Ana", "github"); !errors.Is(err, ErrInvalidAuthProvider) {
		t.Errorf("proveedor invalido: err = %v, se esperaba ErrInvalidAuthProvider", err)
	}
}

func TestNewUserDoesNotAssignIdOrCreatedAt(t *testing.T) {
	u, err := NewUser("ana@example.com", "Ana", AuthGoogle)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	// id y created_at los asigna el backend (capa de datos), no el dominio.
	if u.ID != "" {
		t.Errorf("ID = %q, se esperaba vacio (lo asigna el backend)", u.ID)
	}
	if !u.CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, se esperaba cero (lo asigna el backend)", u.CreatedAt)
	}
}
