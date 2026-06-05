package auth

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/api/idtoken"
)

func verifierWith(payload *idtoken.Payload, err error) *GoogleVerifier {
	return &GoogleVerifier{
		clientID: "client-x.apps.googleusercontent.com",
		validate: func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
			return payload, err
		},
	}
}

func TestGoogleVerifier_ExtractsIdentity(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer: "https://accounts.google.com",
		Claims: map[string]any{"email": "carlos@gmail.com", "name": "Carlos Ruiz"},
	}, nil)
	id, err := v.Verify(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("Verify fallo: %v", err)
	}
	if id.Email != "carlos@gmail.com" || id.Nombre != "Carlos Ruiz" {
		t.Errorf("identidad mal extraida: %+v", id)
	}
}

func TestGoogleVerifier_AcceptsBareIssuer(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer: "accounts.google.com",
		Claims: map[string]any{"email": "c@gmail.com"},
	}, nil)
	if _, err := v.Verify(context.Background(), "idtok"); err != nil {
		t.Errorf("issuer 'accounts.google.com' debe aceptarse: %v", err)
	}
}

func TestGoogleVerifier_RejectsBadIssuer(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer: "https://evil.example.com",
		Claims: map[string]any{"email": "c@gmail.com"},
	}, nil)
	if _, err := v.Verify(context.Background(), "idtok"); err == nil {
		t.Errorf("issuer no confiable debe rechazarse")
	}
}

func TestGoogleVerifier_RejectsMissingEmail(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer: "https://accounts.google.com",
		Claims: map[string]any{"name": "Sin Correo"},
	}, nil)
	if _, err := v.Verify(context.Background(), "idtok"); err == nil {
		t.Errorf("idToken sin email debe rechazarse")
	}
}

func TestGoogleVerifier_PropagatesValidateError(t *testing.T) {
	v := verifierWith(nil, errors.New("firma invalida"))
	if _, err := v.Verify(context.Background(), "idtok"); err == nil {
		t.Errorf("fallo de validate debe propagarse como error")
	}
}
