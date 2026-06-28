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
		Issuer:  "https://accounts.google.com",
		Subject: "sub-123",
		Claims:  map[string]any{"email": "carlos@gmail.com", "name": "Carlos Ruiz", "email_verified": true},
	}, nil)
	id, err := v.Verify(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("Verify fallo: %v", err)
	}
	if id.Sub != "sub-123" || id.Email != "carlos@gmail.com" || id.Nombre != "Carlos Ruiz" || !id.EmailVerified {
		t.Errorf("identidad mal extraida: %+v", id)
	}
}

func TestGoogleVerifier_AcceptsBareIssuer(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer:  "accounts.google.com",
		Subject: "sub-1",
		Claims:  map[string]any{"email": "c@gmail.com"},
	}, nil)
	if _, err := v.Verify(context.Background(), "idtok"); err != nil {
		t.Errorf("issuer 'accounts.google.com' debe aceptarse: %v", err)
	}
}

func TestGoogleVerifier_RejectsMissingSub(t *testing.T) {
	v := verifierWith(&idtoken.Payload{
		Issuer: "https://accounts.google.com",
		Claims: map[string]any{"email": "c@gmail.com"},
	}, nil)
	if _, err := v.Verify(context.Background(), "idtok"); err == nil {
		t.Errorf("idToken sin sub debe rechazarse")
	}
}

// email_verified llega como bool real o, en algunos flujos, como la cadena
// "true"/"false". Se parsea de forma robusta; ausente -> false.
func TestGoogleVerifier_EmailVerifiedParsing(t *testing.T) {
	cases := []struct {
		name  string
		claim any
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"ausente", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{"email": "c@gmail.com"}
			if tc.claim != nil {
				claims["email_verified"] = tc.claim
			}
			v := verifierWith(&idtoken.Payload{
				Issuer:  "https://accounts.google.com",
				Subject: "sub-1",
				Claims:  claims,
			}, nil)
			id, err := v.Verify(context.Background(), "idtok")
			if err != nil {
				t.Fatalf("Verify fallo: %v", err)
			}
			if id.EmailVerified != tc.want {
				t.Errorf("email_verified=%v: got %v, want %v", tc.claim, id.EmailVerified, tc.want)
			}
		})
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
