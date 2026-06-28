//go:build integration

// Pruebas de integracion del adaptador Postgres (07 seccion 9.4). Requieren una
// Postgres real: se activan con -tags=integration y la variable
// TEST_DATABASE_URL. Sin ella, la suite se omite (skip), de modo que el set
// unitario (dominio/app/http) corre sin BD en cada commit.
//
//	TEST_DATABASE_URL='postgres://user@/purpura_test?host=/var/run/postgresql' \
//	    go test -tags=integration ./internal/adapters/postgres/
package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite la integracion Postgres")
	}
	pool, err := NewPool(context.Background(), url)
	if err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	mustMigrate(t, pool)
	t.Cleanup(pool.Close)
	return pool
}

// mustMigrate deja el esquema limpio: dropea y reaplica 0001 + 0002. Como no se
// pasan argumentos, pgx usa el protocolo simple y admite multiples sentencias.
func mustMigrate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS email_verification_tokens, user_credentials, events, users CASCADE;"); err != nil {
		t.Fatalf("drop fallo: %v", err)
	}
	for _, f := range []string{
		"0001_init_schema.up.sql",
		"0002_user_credentials.up.sql",
		"0003_add_google_sub.up.sql",
		"0004_email_verification.up.sql",
	} {
		path := filepath.Join("..", "..", "..", "db", "migrations", f)
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migracion %s fallo: %v", f, err)
		}
	}
}

func seedUser(t *testing.T, pool *pgxpool.Pool, email string) *domain.User {
	t.Helper()
	u, err := domain.NewUser(email, "Tester", domain.AuthPassword)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now().UTC().Truncate(time.Microsecond)
	if err := NewUserRepository(pool).Create(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func newSeedEvent(t *testing.T, userID string, startsAt time.Time, typ domain.EventType) *domain.Event {
	t.Helper()
	e, err := domain.NewEvent(userID, typ,
		domain.Contact{Name: "Maria", Ref: "ref-1"},
		domain.Location{Lat: 19.43, Lng: -99.13, Label: "CDMX"},
		"Revision", startsAt, domain.ReminderTenMinutes)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	e.ID = uuid.NewString()
	e.CreatedAt = now
	e.UpdatedAt = now
	return e
}

func TestIntegration_EventCRUDAndScoping(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	repo := NewEventRepository(pool)
	owner := seedUser(t, pool, "owner@example.com")
	other := seedUser(t, pool, "other@example.com")

	e := newSeedEvent(t, owner.ID, time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC), domain.TypeJunta)
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByID(ctx, owner.ID, e.ID)
	if err != nil {
		t.Fatalf("findByID: %v", err)
	}
	if got.Contact != e.Contact || got.Location != e.Location || !got.StartsAt.Equal(e.StartsAt) {
		t.Errorf("round-trip infiel: %+v vs %+v", got, e)
	}
	if got.Status != domain.StatusPendiente {
		t.Errorf("status persistido = %q", got.Status)
	}

	// Scoping: el ajeno no ve el evento.
	if _, err := repo.FindByID(ctx, other.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("scoping findByID: %v", err)
	}
	if err := repo.Delete(ctx, other.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("scoping delete: %v", err)
	}

	// Update propio.
	e.Description = "Editado"
	e.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.Update(ctx, e); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.FindByID(ctx, owner.ID, e.ID)
	if got.Description != "Editado" {
		t.Errorf("update no persistio: %q", got.Description)
	}

	// Delete propio, luego inexistente.
	if err := repo.Delete(ctx, owner.ID, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, owner.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("delete inexistente: %v", err)
	}
}

func TestIntegration_QueryWindowSortAndPaging(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	repo := NewEventRepository(pool)
	owner := seedUser(t, pool, "q@example.com")

	mk := func(d int, typ domain.EventType) {
		e := newSeedEvent(t, owner.ID, time.Date(2026, 6, d, 10, 0, 0, 0, time.UTC), typ)
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	mk(3, domain.TypeJunta)
	mk(10, domain.TypeCita)
	mk(20, domain.TypeJunta)

	junStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	julStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Frontera End: un evento EXACTAMENTE en End (julio-01 00:00) debe quedar
	// FUERA de [Start, End) (semiabierto).
	boundary := newSeedEvent(t, owner.ID, julStart, domain.TypeJunta)
	if err := repo.Create(ctx, boundary); err != nil {
		t.Fatalf("seed boundary: %v", err)
	}

	base := ports.EventQuery{UserID: owner.ID, HasWindow: true, Start: junStart, End: julStart, SortBy: "starts_at"}

	// Pagina 1 (limit 2) asc.
	page1 := base
	page1.Limit = 2
	evs, total, err := repo.Query(ctx, page1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 3 {
		t.Fatalf("totalItems = %d, quiero 3 (junio: 3,10,20; el 30 cae fuera)", total)
	}
	if len(evs) != 2 || evs[0].StartsAt.Day() != 3 || evs[1].StartsAt.Day() != 10 {
		t.Fatalf("pagina 1 asc incorrecta: %v", days(evs))
	}

	// Orden desc.
	desc := base
	desc.Desc = true
	evs, _, _ = repo.Query(ctx, desc)
	if evs[0].StartsAt.Day() != 20 {
		t.Errorf("desc: primero = dia %d", evs[0].StartsAt.Day())
	}

	// Filtro por tipo junta -> dias 3 y 20.
	jq := base
	jt := domain.TypeJunta
	jq.Type = &jt
	_, total, _ = repo.Query(ctx, jq)
	if total != 2 {
		t.Errorf("filtro junta: total = %d, quiero 2", total)
	}
}

func TestIntegration_UserCredentialAtomicAndCascade(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	events := NewEventRepository(pool)

	u, err := domain.NewUser("ana@example.com", "Ana", domain.AuthPassword)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now().UTC()
	if err := users.CreateWithPassword(ctx, u, "hash:secreta"); err != nil {
		t.Fatalf("createWithPassword: %v", err)
	}

	hash, err := users.GetPasswordHash(ctx, u.ID)
	if err != nil || hash != "hash:secreta" {
		t.Fatalf("getPasswordHash = %q, err=%v", hash, err)
	}

	// Email duplicado case-insensitive -> ErrEmailTaken (con rollback atomico).
	dup, _ := domain.NewUser("ANA@example.com", "Ana2", domain.AuthPassword)
	dup.ID = uuid.NewString()
	dup.CreatedAt = time.Now().UTC()
	if err := users.CreateWithPassword(ctx, dup, "hash:x"); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("dup email: %v", err)
	}
	if _, err := users.FindByID(ctx, dup.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("el usuario duplicado no debe haberse creado (rollback): %v", err)
	}

	// FindByEmail case-insensitive.
	if _, err := users.FindByEmail(ctx, "Ana@Example.com"); err != nil {
		t.Errorf("findByEmail case-insensitive: %v", err)
	}

	// ON DELETE CASCADE: borrar el usuario elimina sus eventos.
	e := newSeedEvent(t, u.ID, time.Now().UTC(), domain.TypeOtros)
	if err := events.Create(ctx, e); err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := events.FindByID(ctx, u.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("cascade: el evento debio borrarse con el usuario: %v", err)
	}
}

func TestIntegration_DeleteAccountCascades(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	events := NewEventRepository(pool)

	// Cuenta password con credencial + un evento propio.
	u, err := domain.NewUser("borrar@example.com", "Borrar", domain.AuthPassword)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now().UTC()
	if err := users.CreateWithPassword(ctx, u, "hash:secreta"); err != nil {
		t.Fatalf("createWithPassword: %v", err)
	}
	e := newSeedEvent(t, u.ID, time.Now().UTC(), domain.TypeOtros)
	if err := events.Create(ctx, e); err != nil {
		t.Fatalf("create event: %v", err)
	}

	// DeleteAccount borra al usuario y, por ON DELETE CASCADE, su credencial y eventos.
	if err := users.DeleteAccount(ctx, u.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := users.FindByID(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("usuario debio borrarse: %v", err)
	}
	if _, err := users.GetPasswordHash(ctx, u.ID); !errors.Is(err, domain.ErrInvalidCredential) {
		t.Errorf("cascade: la credencial debio caer con el usuario: %v", err)
	}
	if _, err := events.FindByID(ctx, u.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("cascade: el evento debio borrarse con el usuario: %v", err)
	}

	// Segundo borrado de una cuenta ya inexistente -> ErrUserNotFound (0 filas).
	if err := users.DeleteAccount(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("borrado idempotente -> ErrUserNotFound, obtuve %v", err)
	}
}

func TestIntegration_DeleteAccountGoogleNoCredential(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	events := NewEventRepository(pool)

	// Cuenta Google: NO tiene fila en user_credentials (solo las password la tienen). El borrado
	// debe funcionar igual y cascadear sus eventos, aunque no haya credencial que cascadear.
	u, err := domain.NewUser("google@example.com", "Goo", domain.AuthGoogle)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now().UTC()
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("create google user: %v", err)
	}
	e := newSeedEvent(t, u.ID, time.Now().UTC(), domain.TypeOtros)
	if err := events.Create(ctx, e); err != nil {
		t.Fatalf("create event: %v", err)
	}

	if err := users.DeleteAccount(ctx, u.ID); err != nil {
		t.Fatalf("DeleteAccount (google): %v", err)
	}
	if _, err := users.FindByID(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("usuario google debio borrarse: %v", err)
	}
	if _, err := events.FindByID(ctx, u.ID, e.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Errorf("cascade: el evento debio borrarse con el usuario google: %v", err)
	}
}

func TestIntegration_GoogleSubLinkLifecycle(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)

	u := seedUser(t, pool, "link@example.com") // cuenta password, google_sub NULL
	// Sin vincular: SELECT * trae GoogleSub nil y no se halla por sub.
	if got, err := users.FindByID(ctx, u.ID); err != nil || got.GoogleSub != nil {
		t.Fatalf("recien creada no debe tener sub: sub=%v err=%v", got.GoogleSub, err)
	}
	if _, err := users.FindByGoogleSub(ctx, "sub-link"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("sub inexistente -> ErrUserNotFound, obtuve %v", err)
	}

	// Vincular y hallar por sub.
	if err := users.LinkGoogleSub(ctx, u.ID, "sub-link"); err != nil {
		t.Fatalf("LinkGoogleSub: %v", err)
	}
	got, err := users.FindByGoogleSub(ctx, "sub-link")
	if err != nil || got.ID != u.ID || got.GoogleSub == nil || *got.GoogleSub != "sub-link" {
		t.Fatalf("tras vincular debe hallarse por sub con google_sub mapeado: %+v err=%v", got, err)
	}

	// Unicidad: otro usuario no puede tomar el mismo sub.
	other := seedUser(t, pool, "other-link@example.com")
	if err := users.LinkGoogleSub(ctx, other.ID, "sub-link"); !errors.Is(err, domain.ErrGoogleLinkConflict) {
		t.Fatalf("sub duplicado -> ErrGoogleLinkConflict (23505), obtuve %v", err)
	}

	// Desvincular deja NULL e invisibiliza por sub.
	if err := users.ClearGoogleSub(ctx, u.ID); err != nil {
		t.Fatalf("ClearGoogleSub: %v", err)
	}
	if _, err := users.FindByGoogleSub(ctx, "sub-link"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("tras desvincular no debe hallarse por sub, obtuve %v", err)
	}
	// Y ahora el sub queda libre para otra cuenta.
	if err := users.LinkGoogleSub(ctx, other.ID, "sub-link"); err != nil {
		t.Errorf("el sub liberado debe poder re-vincularse: %v", err)
	}
}

func TestIntegration_LinkClearGoogleSubUnknownUser(t *testing.T) {
	pool := mustPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	ghost := uuid.NewString()
	if err := users.LinkGoogleSub(ctx, ghost, "sub-x"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("LinkGoogleSub a usuario inexistente -> ErrUserNotFound, obtuve %v", err)
	}
	if err := users.ClearGoogleSub(ctx, ghost); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("ClearGoogleSub a usuario inexistente -> ErrUserNotFound, obtuve %v", err)
	}
}

func days(es []domain.Event) []int {
	out := make([]int, len(es))
	for i, e := range es {
		out[i] = e.StartsAt.Day()
	}
	return out
}

func TestIntegration_SetEmailVerified(t *testing.T) {
	pool := mustPool(t)
	repo := NewUserRepository(pool)
	ctx := context.Background()

	u := seedUser(t, pool, "verify@example.com")
	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.EmailVerified {
		t.Fatalf("una cuenta de contrasena nace sin verificar")
	}

	if err := repo.SetEmailVerified(ctx, u.ID); err != nil {
		t.Fatalf("SetEmailVerified: %v", err)
	}
	got, _ = repo.FindByID(ctx, u.ID)
	if !got.EmailVerified {
		t.Errorf("el correo debe quedar verificado")
	}
	// Idempotente.
	if err := repo.SetEmailVerified(ctx, u.ID); err != nil {
		t.Errorf("SetEmailVerified idempotente fallo: %v", err)
	}
	// Usuario inexistente -> ErrUserNotFound.
	if err := repo.SetEmailVerified(ctx, uuid.NewString()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("usuario inexistente -> ErrUserNotFound, got %v", err)
	}
}

func TestIntegration_VerificationTokenLifecycle(t *testing.T) {
	pool := mustPool(t)
	users := NewUserRepository(pool)
	repo := NewVerificationTokenRepository(pool)
	ctx := context.Background()

	u := seedUser(t, pool, "vtoken@example.com")
	now := time.Now().UTC().Truncate(time.Second)
	tok := &ports.VerificationToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: "hash-deadbeef",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create token: %v", err)
	}

	got, err := repo.FindByHash(ctx, "hash-deadbeef")
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if got.UserID != u.ID || got.UsedAt != nil {
		t.Errorf("token recien creado mal: userID=%q usedAt=%v", got.UserID, got.UsedAt)
	}

	// MarkUsed es de un solo uso: el segundo intento ve 0 filas (false).
	ok, err := repo.MarkUsed(ctx, tok.ID, now)
	if err != nil || !ok {
		t.Fatalf("primer MarkUsed: ok=%v err=%v", ok, err)
	}
	ok2, err := repo.MarkUsed(ctx, tok.ID, now)
	if err != nil {
		t.Fatalf("segundo MarkUsed: %v", err)
	}
	if ok2 {
		t.Errorf("el segundo MarkUsed debe ser false (un solo uso)")
	}
	got2, _ := repo.FindByHash(ctx, "hash-deadbeef")
	if got2.UsedAt == nil {
		t.Errorf("UsedAt debe estar seteado tras MarkUsed")
	}

	// Hash inexistente -> ErrInvalidVerificationToken.
	if _, err := repo.FindByHash(ctx, "no-existe"); !errors.Is(err, domain.ErrInvalidVerificationToken) {
		t.Errorf("hash inexistente -> ErrInvalidVerificationToken, got %v", err)
	}

	// Cascade: al borrar el usuario, sus tokens caen (FK ON DELETE CASCADE).
	if err := users.DeleteAccount(ctx, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := repo.FindByHash(ctx, "hash-deadbeef"); !errors.Is(err, domain.ErrInvalidVerificationToken) {
		t.Errorf("tras borrar el usuario, el token debe desaparecer por cascade, got %v", err)
	}
}
