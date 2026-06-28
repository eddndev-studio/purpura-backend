//go:build integration

// Prueba de humo end-to-end (07 seccion 9.5): ensambla el stack REAL (router +
// casos de uso + repos Postgres + JWT/bcrypt reales) contra una Postgres real y
// ejercita el flujo completo via HTTP. Se activa con -tags=integration y
// TEST_DATABASE_URL. Correr en serie con el resto de la integracion: -p 1.
package httpadapter_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddndev-studio/purpura-backend/internal/adapters/auth"
	httpadapter "github.com/eddndev-studio/purpura-backend/internal/adapters/http"
	"github.com/eddndev-studio/purpura-backend/internal/adapters/postgres"
	"github.com/eddndev-studio/purpura-backend/internal/adapters/sys"
	"github.com/eddndev-studio/purpura-backend/internal/app"
)

func mustE2EServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el e2e")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	migrate(t, pool)

	tokens, err := auth.NewJWTService(auth.JWTConfig{
		SigningMethod: "HS256", Secret: "e2e-secret-0123456789",
		Issuer: "purpura-backend", Audience: "purpura-app", TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}
	eventSvc := &app.EventService{
		Events: postgres.NewEventRepository(pool), Clock: sys.NewClock(), IDs: sys.NewUUIDGenerator(),
	}
	authSvc := &app.AuthService{
		Users:  postgres.NewUserRepository(pool),
		Tokens: tokens, Google: auth.NewGoogleVerifier("client-x"),
		Hasher: auth.NewBcryptHasher(4), Clock: sys.NewClock(), IDs: sys.NewUUIDGenerator(),
	}
	router := httpadapter.NewRouter(httpadapter.Deps{
		Events: eventSvc, Auth: authSvc, Tokens: tokens, Pinger: pool,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	t.Cleanup(pool.Close)
	return srv, pool
}

func migrate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS email_verification_tokens, user_credentials, events, users CASCADE;"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	for _, f := range []string{"0001_init_schema.up.sql", "0002_user_credentials.up.sql", "0003_add_google_sub.up.sql", "0004_email_verification.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migrate %s: %v", f, err)
		}
	}
}

func TestE2E_FullFlow(t *testing.T) {
	srv, _ := mustE2EServer(t)
	c := srv.Client()

	// 1. Registro -> accessToken.
	reg := postJSON(t, c, srv.URL+"/api/v1/auth/register", "",
		`{"email":"e2e@example.com","nombre":"E2E","password":"S3guroPurpura!"}`)
	assertStatus(t, reg, 201)
	token := decode(t, reg)["accessToken"].(string)
	if token == "" {
		t.Fatal("accessToken vacio")
	}

	// 2. Crear evento.
	create := postJSON(t, c, srv.URL+"/api/v1/events", token,
		`{"eventType":"junta","contactName":"Maria","locationLat":19.43,"locationLng":-99.13,
		  "description":"Revision","startsAt":"2026-06-10T15:30:00Z","reminderType":"ten_minutes_before"}`)
	assertStatus(t, create, 201)
	ev := decode(t, create)
	id, _ := ev["id"].(string)
	if id == "" || ev["eventStatus"] != "pendiente" {
		t.Fatalf("evento creado mal: %v", ev)
	}

	// 3. Obtenerlo.
	get := doReq(t, c, http.MethodGet, srv.URL+"/api/v1/events/"+id, token, "")
	assertStatus(t, get, 200)

	// 4. Consultarlo por mes.
	q := doReq(t, c, http.MethodGet, srv.URL+"/api/v1/events?mode=por_mes&year=2026&month=6", token, "")
	assertStatus(t, q, 200)
	data := decode(t, q)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("query devolvio %d eventos, quiero 1", len(data))
	}

	// 5. Cambiar estatus.
	st := doReq(t, c, http.MethodPatch, srv.URL+"/api/v1/events/"+id+"/status", token, `{"eventStatus":"realizado"}`)
	assertStatus(t, st, 200)
	if decode(t, st)["eventStatus"] != "realizado" {
		t.Errorf("estatus no cambio")
	}

	// 6. Borrarlo y confirmar 404.
	del := doReq(t, c, http.MethodDelete, srv.URL+"/api/v1/events/"+id, token, "")
	assertStatus(t, del, 204)
	gone := doReq(t, c, http.MethodGet, srv.URL+"/api/v1/events/"+id, token, "")
	assertStatus(t, gone, 404)

	// 7. Health.
	health := doReq(t, c, http.MethodGet, srv.URL+"/health", "", "")
	assertStatus(t, health, 200)
}

// ---- helpers HTTP ----

func doReq(t *testing.T, c *http.Client, method, url, token, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func postJSON(t *testing.T, c *http.Client, url, token, body string) *http.Response {
	return doReq(t, c, http.MethodPost, url, token, body)
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, quiero %d (%s)", resp.StatusCode, want, string(b))
	}
}

func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}
