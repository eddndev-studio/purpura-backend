package httpadapter

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// fakeVerification implementa VerificationUseCases: registra los argumentos y
// devuelve errores configurables.
type fakeVerification struct {
	requestErr error
	confirmErr error

	gotRequestID  string
	gotConfirmTok string
}

func (f *fakeVerification) RequestVerification(_ context.Context, userID string) error {
	f.gotRequestID = userID
	return f.requestErr
}

func (f *fakeVerification) ConfirmVerification(_ context.Context, rawToken string) error {
	f.gotConfirmTok = rawToken
	return f.confirmErr
}

func TestMe_200ReturnsUserWithEmailVerified(t *testing.T) {
	auth := &fakeAuth{meUser: &domain.User{
		ID: "user-1", Email: "ana@example.com", Nombre: "Ana",
		AuthProvider: domain.AuthPassword, EmailVerified: false,
	}}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"emailVerified":false`) {
		t.Errorf("la respuesta debe incluir emailVerified:false: %s", rec.Body.String())
	}
	if auth.gotMeID != "user-1" {
		t.Errorf("userID = %q, quiero user-1 (del token)", auth.gotMeID)
	}
}

func TestMe_NoToken_401(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
}

func TestRequestVerification_202(t *testing.T) {
	v := &fakeVerification{}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: v})
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/request", "", true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, quiero 202 (%s)", rec.Code, rec.Body.String())
	}
	if v.gotRequestID != "user-1" {
		t.Errorf("userID = %q, quiero user-1 (del token)", v.gotRequestID)
	}
}

func TestRequestVerification_NoToken_401(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: &fakeVerification{}})
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/request", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
}

func TestConfirmVerification_204(t *testing.T) {
	v := &fakeVerification{}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: v})
	// Publico: sin token de sesion. El token del cuerpo ES la credencial.
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/confirm", `{"token":"raw-x"}`, false)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, quiero 204 (%s)", rec.Code, rec.Body.String())
	}
	if v.gotConfirmTok != "raw-x" {
		t.Errorf("token = %q, quiero raw-x (del cuerpo)", v.gotConfirmTok)
	}
}

func TestConfirmVerification_EmptyToken_400(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: &fakeVerification{}})
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/confirm", `{"token":""}`, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400", rec.Code)
	}
}

func TestConfirmVerification_Invalid_400(t *testing.T) {
	v := &fakeVerification{confirmErr: domain.ErrInvalidVerificationToken}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: v})
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/confirm", `{"token":"bad"}`, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_verification_token"`) {
		t.Errorf("quiero code invalid_verification_token: %s", rec.Body.String())
	}
}

func TestConfirmVerification_Expired_410(t *testing.T) {
	v := &fakeVerification{confirmErr: domain.ErrVerificationTokenExpired}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Verification: v})
	rec := do(h, http.MethodPost, "/api/v1/auth/verify-email/confirm", `{"token":"old"}`, false)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, quiero 410 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"verification_token_expired"`) {
		t.Errorf("quiero code verification_token_expired: %s", rec.Body.String())
	}
}
