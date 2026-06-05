package httpadapter

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/app"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// ---- fakes de los casos de uso (07 seccion 9.3) ----

type fakeEvents struct {
	ev      *domain.Event
	evErr   error
	list    app.QueryEventsResult
	listErr error
	delErr  error
	export  app.ExportResult
	expErr  error
	imp     app.ImportSummary
	impErr  error

	gotUserID string
	gotCreate app.CreateEventInput
	gotQuery  app.QueryEventsInput
	gotPatch  domain.EventPatch
	gotStatus domain.EventStatus
}

func (f *fakeEvents) CreateEvent(_ context.Context, userID string, in app.CreateEventInput) (*domain.Event, error) {
	f.gotUserID, f.gotCreate = userID, in
	return f.ev, f.evErr
}
func (f *fakeEvents) GetEvent(_ context.Context, userID, _ string) (*domain.Event, error) {
	f.gotUserID = userID
	return f.ev, f.evErr
}
func (f *fakeEvents) QueryEvents(_ context.Context, userID string, in app.QueryEventsInput) (app.QueryEventsResult, error) {
	f.gotUserID, f.gotQuery = userID, in
	return f.list, f.listErr
}
func (f *fakeEvents) UpdateEvent(_ context.Context, userID, _ string, in app.UpdateEventInput) (*domain.Event, error) {
	f.gotUserID, f.gotPatch = userID, in.Patch
	return f.ev, f.evErr
}
func (f *fakeEvents) ChangeStatus(_ context.Context, userID, _ string, s domain.EventStatus) (*domain.Event, error) {
	f.gotUserID, f.gotStatus = userID, s
	return f.ev, f.evErr
}
func (f *fakeEvents) DeleteEvent(_ context.Context, userID, _ string) error {
	f.gotUserID = userID
	return f.delErr
}
func (f *fakeEvents) ExportEvents(_ context.Context, userID string, in app.QueryEventsInput) (app.ExportResult, error) {
	f.gotUserID, f.gotQuery = userID, in
	return f.export, f.expErr
}
func (f *fakeEvents) ImportEvents(_ context.Context, userID string, _ app.ImportInput) (app.ImportSummary, error) {
	f.gotUserID = userID
	return f.imp, f.impErr
}

type fakeAuth struct {
	res app.AuthResult
	err error
}

func (f *fakeAuth) Register(context.Context, app.RegisterInput) (app.AuthResult, error) {
	return f.res, f.err
}
func (f *fakeAuth) Login(context.Context, app.LoginInput) (app.AuthResult, error) {
	return f.res, f.err
}
func (f *fakeAuth) AuthenticateWithGoogle(context.Context, string) (app.AuthResult, error) {
	return f.res, f.err
}

type fakeToken struct {
	claims ports.Claims
	err    error
}

func (f fakeToken) Verify(context.Context, string) (ports.Claims, error) {
	return f.claims, f.err
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// ---- helpers ----

func sampleEvent() *domain.Event {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	return &domain.Event{
		ID: "ev-1", UserID: "user-1", Type: domain.TypeJunta,
		Contact:     domain.Contact{Name: "Maria", Ref: "ref-1"},
		Location:    domain.Location{Lat: 19.43, Lng: -99.13, Label: "CDMX"},
		Description: "Revision", StartsAt: time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC),
		Status: domain.StatusPendiente, Reminder: domain.ReminderTenMinutes,
		CreatedAt: now, UpdatedAt: now,
	}
}

func newServer(d Deps) http.Handler {
	if d.Tokens == nil {
		d.Tokens = fakeToken{claims: ports.Claims{Subject: "user-1"}}
	}
	d.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(d)
}

func do(h http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth {
		req.Header.Set("Authorization", "Bearer good")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body.String())
	}
	return m
}

// ---- tests ----

func TestCreateEvent_201CamelCaseAndOwnerFromToken(t *testing.T) {
	ev := &fakeEvents{ev: sampleEvent()}
	h := newServer(Deps{Events: ev, Auth: &fakeAuth{}})

	body := `{"eventType":"junta","contactName":"Maria","locationLat":19.43,"locationLng":-99.13,
		"description":"Revision","startsAt":"2026-06-10T15:30:00Z","reminderType":"ten_minutes_before",
		"userId":"INTRUSO","eventStatus":"realizado"}`
	rec := do(h, http.MethodPost, "/api/v1/events", body, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiero 201 (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}
	m := decodeBody(t, rec)
	for _, k := range []string{"eventType", "eventStatus", "reminderType", "contactName", "startsAt", "userId"} {
		if _, ok := m[k]; !ok {
			t.Errorf("falta clave camelCase %q en %v", k, m)
		}
	}
	// El propietario viene del sub del token, no del cuerpo.
	if ev.gotUserID != "user-1" {
		t.Errorf("userID = %q, quiero user-1 (del token)", ev.gotUserID)
	}
	// userId/eventStatus del cuerpo se ignoran (no llegan al input).
	if ev.gotCreate.Type != domain.TypeJunta {
		t.Errorf("input mal decodificado: %+v", ev.gotCreate)
	}
}

func TestProtected_NoToken_401Problem(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodGet, "/api/v1/events", "", false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("content-type = %q, quiero problem+json", ct)
	}
	if decodeBody(t, rec)["code"] != "unauthorized" {
		t.Errorf("code = %v", decodeBody(t, rec)["code"])
	}
}

func TestProtected_InvalidToken_401(t *testing.T) {
	h := newServer(Deps{
		Events: &fakeEvents{}, Auth: &fakeAuth{},
		Tokens: fakeToken{err: context.DeadlineExceeded},
	})
	rec := do(h, http.MethodGet, "/api/v1/events", "", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiero 401", rec.Code)
	}
}

func TestGetEvent_ForeignMapsTo404(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{evErr: domain.ErrEventNotFound}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodGet, "/api/v1/events/ev-9", "", true)
	if rec.Code != http.StatusNotFound || decodeBody(t, rec)["code"] != "event_not_found" {
		t.Fatalf("status/code = %d/%v", rec.Code, decodeBody(t, rec)["code"])
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		evErr    error
		wantCode int
		wantStr  string
	}{
		{"empty description", domain.ErrEmptyDescription, 422, "empty_description"},
		{"invalid location", domain.ErrInvalidLocation, 422, "invalid_location"},
		{"validation", app.ErrValidation, 422, "validation_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newServer(Deps{Events: &fakeEvents{evErr: tc.evErr}, Auth: &fakeAuth{}})
			body := `{"eventType":"junta","locationLat":0,"locationLng":0,"description":"x","startsAt":"2026-06-10T15:30:00Z","reminderType":"none"}`
			rec := do(h, http.MethodPost, "/api/v1/events", body, true)
			if rec.Code != tc.wantCode || decodeBody(t, rec)["code"] != tc.wantStr {
				t.Fatalf("status/code = %d/%v, quiero %d/%s", rec.Code, decodeBody(t, rec)["code"], tc.wantCode, tc.wantStr)
			}
		})
	}
}

func TestMalformedJSON_400(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPost, "/api/v1/events", `{ not json`, true)
	if rec.Code != http.StatusBadRequest || decodeBody(t, rec)["code"] != "bad_request" {
		t.Fatalf("status/code = %d/%v", rec.Code, decodeBody(t, rec)["code"])
	}
}

func TestUpdate_PatchDistinguishesAbsentFromEmpty(t *testing.T) {
	ev := &fakeEvents{ev: sampleEvent()}
	h := newServer(Deps{Events: ev, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPatch, "/api/v1/events/ev-1", `{"contactRef":""}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// contactRef:"" presente => Contact no nil con Ref vacio; otros campos nil.
	if ev.gotPatch.Contact == nil || ev.gotPatch.Contact.Ref != "" {
		t.Errorf("patch contact = %+v, quiero presente con Ref vacio", ev.gotPatch.Contact)
	}
	if ev.gotPatch.Description != nil || ev.gotPatch.Type != nil {
		t.Errorf("campos ausentes deben ser nil: %+v", ev.gotPatch)
	}
}

func TestChangeStatus_PassesStatus(t *testing.T) {
	ev := &fakeEvents{ev: sampleEvent()}
	h := newServer(Deps{Events: ev, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPatch, "/api/v1/events/ev-1/status", `{"eventStatus":"realizado"}`, true)
	if rec.Code != http.StatusOK || ev.gotStatus != domain.StatusRealizado {
		t.Fatalf("status = %d, gotStatus = %q", rec.Code, ev.gotStatus)
	}
}

func TestDelete_204(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodDelete, "/api/v1/events/ev-1", "", true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, quiero 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 no debe tener cuerpo: %q", rec.Body.String())
	}
}

func TestQuery_200EnvelopeAndParams(t *testing.T) {
	ev := &fakeEvents{list: app.QueryEventsResult{
		Events: []domain.Event{*sampleEvent()},
		Page:   1, PageSize: 20, TotalItems: 1, TotalPages: 1, Sort: "startsAt:asc",
	}}
	h := newServer(Deps{Events: ev, Auth: &fakeAuth{}})
	rec := do(h, http.MethodGet, "/api/v1/events?mode=por_mes&year=2026&month=6&type=junta", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if _, ok := m["data"]; !ok {
		t.Errorf("falta data")
	}
	if _, ok := m["pagination"]; !ok {
		t.Errorf("falta pagination")
	}
	if ev.gotQuery.Mode != "por_mes" || ev.gotQuery.Year != 2026 || ev.gotQuery.Month != 6 {
		t.Errorf("query params mal parseados: %+v", ev.gotQuery)
	}
	if ev.gotQuery.Type == nil || *ev.gotQuery.Type != domain.TypeJunta {
		t.Errorf("type filtro mal: %+v", ev.gotQuery.Type)
	}
}

func TestQuery_BadNumericParam_400(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodGet, "/api/v1/events?page=abc", "", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400", rec.Code)
	}
}

func TestAuth_RegisterLoginGoogle(t *testing.T) {
	res := app.AuthResult{
		Token: ports.IssuedToken{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 86400},
		User:  &domain.User{ID: "u1", Email: "ana@example.com", Nombre: "Ana", AuthProvider: domain.AuthPassword},
	}
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{res: res}})

	rec := do(h, http.MethodPost, "/api/v1/auth/register", `{"email":"ana@example.com","nombre":"Ana","password":"S3guroPurpura!"}`, false)
	if rec.Code != http.StatusCreated {
		t.Errorf("register status = %d, quiero 201", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["accessToken"] != "tok" || m["tokenType"] != "Bearer" {
		t.Errorf("register body mal: %v", m)
	}

	rec = do(h, http.MethodPost, "/api/v1/auth/login", `{"email":"ana@example.com","password":"S3guroPurpura!"}`, false)
	if rec.Code != http.StatusOK {
		t.Errorf("login status = %d, quiero 200", rec.Code)
	}

	rec = do(h, http.MethodPost, "/api/v1/auth/google", `{"idToken":"x"}`, false)
	if rec.Code != http.StatusOK {
		t.Errorf("google status = %d, quiero 200", rec.Code)
	}
}

func TestLogin_InvalidCredential_401(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{err: domain.ErrInvalidCredential}})
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"email":"a@x.com","password":"mala"}`, false)
	if rec.Code != http.StatusUnauthorized || decodeBody(t, rec)["code"] != "invalid_credential" {
		t.Fatalf("status/code = %d/%v", rec.Code, decodeBody(t, rec)["code"])
	}
}

func TestRegister_EmailTaken_409(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{err: domain.ErrEmailTaken}})
	rec := do(h, http.MethodPost, "/api/v1/auth/register", `{"email":"a@x.com","nombre":"A","password":"S3guroPurpura!"}`, false)
	if rec.Code != http.StatusConflict || decodeBody(t, rec)["code"] != "email_taken" {
		t.Fatalf("status/code = %d/%v", rec.Code, decodeBody(t, rec)["code"])
	}
}

func TestGoogle_MissingIdToken_400(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	rec := do(h, http.MethodPost, "/api/v1/auth/google", `{}`, false)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400", rec.Code)
	}
}

func TestImport_PayloadTooLarge_413(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, MaxBodyBytes: 32})
	big := `{"mode":"partial","events":[` + strings.Repeat(`{"eventType":"otros"},`, 50) + `{}]}`
	rec := do(h, http.MethodPost, "/api/v1/events/import", big, true)
	if rec.Code != http.StatusRequestEntityTooLarge || decodeBody(t, rec)["code"] != "payload_too_large" {
		t.Fatalf("status/code = %d/%v", rec.Code, decodeBody(t, rec)["code"])
	}
}

func TestHealth(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Pinger: fakePinger{}})
	rec := do(h, http.MethodGet, "/health", "", false)
	if rec.Code != http.StatusOK || decodeBody(t, rec)["status"] != "ok" {
		t.Errorf("health sano: status = %d body = %v", rec.Code, rec.Body.String())
	}

	down := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}, Pinger: fakePinger{err: context.DeadlineExceeded}})
	rec = do(down, http.MethodGet, "/health", "", false)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("health caido: status = %d, quiero 503", rec.Code)
	}
}

func TestRouting_NotFoundAndMethodNotAllowed(t *testing.T) {
	h := newServer(Deps{Events: &fakeEvents{}, Auth: &fakeAuth{}})
	if rec := do(h, http.MethodGet, "/api/v1/nope", "", true); rec.Code != http.StatusNotFound {
		t.Errorf("ruta inexistente status = %d, quiero 404", rec.Code)
	}
	// /health solo admite GET.
	if rec := do(h, http.MethodPost, "/health", "", false); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("metodo no permitido status = %d, quiero 405", rec.Code)
	}
}
