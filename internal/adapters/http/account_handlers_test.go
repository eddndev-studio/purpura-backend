package httpadapter

import (
	"net/http"
	"strings"
	"testing"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

func linkedUser(sub string) *domain.User {
	return &domain.User{ID: "user-1", Email: "ana@example.com", Nombre: "Ana", AuthProvider: domain.AuthPassword, GoogleSub: &sub}
}

func TestLinkGoogle_200ReturnsGoogleLinked(t *testing.T) {
	auth := &fakeAuth{linkUser: linkedUser("sub-ana")}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodPost, "/api/v1/account/link-google", `{"idToken":"tok-x"}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"googleLinked":true`) {
		t.Errorf("la respuesta debe incluir googleLinked:true: %s", rec.Body.String())
	}
	// El id sale del sub del token; el idToken viene del cuerpo.
	if auth.gotLinkID != "user-1" || auth.gotLinkTok != "tok-x" {
		t.Errorf("id/idToken mal propagados: id=%q tok=%q", auth.gotLinkID, auth.gotLinkTok)
	}
}

func TestLinkGoogle_NoToken_401(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPost, "/api/v1/account/link-google", `{"idToken":"tok-x"}`, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
}

func TestLinkGoogle_EmptyIdToken_400(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPost, "/api/v1/account/link-google", `{"idToken":""}`, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400", rec.Code)
	}
}

func TestLinkGoogle_Conflict_409(t *testing.T) {
	auth := &fakeAuth{linkErr: domain.ErrGoogleLinkConflict}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodPost, "/api/v1/account/link-google", `{"idToken":"tok-x"}`, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, quiero 409 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"google_link_conflict"`) {
		t.Errorf("quiero code google_link_conflict: %s", rec.Body.String())
	}
}

func TestLinkGoogle_InvalidToken_400(t *testing.T) {
	auth := &fakeAuth{linkErr: domain.ErrInvalidGoogleToken}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodPost, "/api/v1/account/link-google", `{"idToken":"bad"}`, true)
	// idToken invalido en una peticion autenticada -> 400 (no 401): el cliente no
	// debe confundirlo con sesion expirada.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_google_token"`) {
		t.Errorf("quiero code invalid_google_token: %s", rec.Body.String())
	}
}

func TestUnlinkGoogle_200ReturnsUnlinked(t *testing.T) {
	auth := &fakeAuth{unlinkUser: &domain.User{ID: "user-1", Email: "ana@example.com", Nombre: "Ana", AuthProvider: domain.AuthPassword}}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodDelete, "/api/v1/account/link-google", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"googleLinked":false`) {
		t.Errorf("la respuesta debe incluir googleLinked:false: %s", rec.Body.String())
	}
	if auth.gotUnlinkID != "user-1" {
		t.Errorf("userID = %q, quiero user-1 (del token)", auth.gotUnlinkID)
	}
}

func TestUnlinkGoogle_NoPassword_409(t *testing.T) {
	auth := &fakeAuth{unlinkErr: domain.ErrCannotUnlinkGoogle}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: auth})
	rec := do(h, http.MethodDelete, "/api/v1/account/link-google", "", true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, quiero 409 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"cannot_unlink_google"`) {
		t.Errorf("quiero code cannot_unlink_google: %s", rec.Body.String())
	}
}

func TestUnlinkGoogle_NoToken_401(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodDelete, "/api/v1/account/link-google", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
}
