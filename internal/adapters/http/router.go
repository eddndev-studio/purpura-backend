// Package httpadapter es el adaptador driving (chi). Traduce HTTP <-> casos de
// uso: decodifica el cuerpo a DTOs de app, invoca el caso de uso, codifica la
// salida camelCase y mapea errores a problem+json. No contiene logica de
// negocio. Vive en internal/adapters/http pero se llama httpadapter para no
// chocar con net/http.
package httpadapter

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/eddndev-studio/purpura-backend/internal/app"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// EventUseCases es el subconjunto de EventService que consume el adaptador. Se
// declara como interfaz para poder montar los handlers contra fakes (07 9.3).
type EventUseCases interface {
	CreateEvent(ctx context.Context, userID string, in app.CreateEventInput) (*domain.Event, error)
	GetEvent(ctx context.Context, userID, id string) (*domain.Event, error)
	QueryEvents(ctx context.Context, userID string, in app.QueryEventsInput) (app.QueryEventsResult, error)
	UpdateEvent(ctx context.Context, userID, id string, in app.UpdateEventInput) (*domain.Event, error)
	ChangeStatus(ctx context.Context, userID, id string, status domain.EventStatus) (*domain.Event, error)
	DeleteEvent(ctx context.Context, userID, id string) error
	ExportEvents(ctx context.Context, userID string, in app.QueryEventsInput) (app.ExportResult, error)
	ImportEvents(ctx context.Context, userID string, in app.ImportInput) (app.ImportSummary, error)
}

// AuthUseCases es el subconjunto de AuthService que consume el adaptador.
type AuthUseCases interface {
	Register(ctx context.Context, in app.RegisterInput) (app.AuthResult, error)
	Login(ctx context.Context, in app.LoginInput) (app.AuthResult, error)
	AuthenticateWithGoogle(ctx context.Context, idToken string) (app.AuthResult, error)
	LinkGoogle(ctx context.Context, userID, idToken string) (*domain.User, error)
	UnlinkGoogle(ctx context.Context, userID string) (*domain.User, error)
	Me(ctx context.Context, userID string) (*domain.User, error)
	DeleteAccount(ctx context.Context, userID string) error
}

// VerificationUseCases es el subconjunto de VerificationService que consume el
// adaptador (verificacion de correo).
type VerificationUseCases interface {
	RequestVerification(ctx context.Context, userID string) error
	ConfirmVerification(ctx context.Context, rawToken string) error
}

// TokenVerifier verifica el JWT del header (lo cumple ports.TokenService).
type TokenVerifier interface {
	Verify(ctx context.Context, accessToken string) (ports.Claims, error)
}

// Pinger comprueba la salud de la BD (lo cumple *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps son las dependencias del router (composition root las provee).
type Deps struct {
	Events       EventUseCases
	Auth         AuthUseCases
	Verification VerificationUseCases
	Tokens       TokenVerifier
	Pinger       Pinger
	CORSOrigins  []string
	MaxBodyBytes int64
	Logger       *slog.Logger
}

const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// NewRouter ensambla el chi.Router con middleware y rutas (04 seccion 5).
func NewRouter(d Deps) http.Handler {
	if d.MaxBodyBytes <= 0 {
		d.MaxBodyBytes = defaultMaxBodyBytes
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(d.recoverer)
	r.Use(d.logging)
	r.Use(d.cors)
	r.NotFound(handleNotFound)
	r.MethodNotAllowed(handleMethodNotAllowed)

	r.Get("/health", d.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			// Publicas: la credencial viaja en el cuerpo (register/login/google) o
			// el token de un solo uso ES la credencial (verify-email/confirm).
			r.Post("/register", d.handleRegister)
			r.Post("/login", d.handleLogin)
			r.Post("/google", d.handleGoogle)
			r.Post("/verify-email/confirm", d.handleConfirmVerification)

			// Autenticadas dentro de /auth (requieren JWT del usuario).
			r.Group(func(r chi.Router) {
				r.Use(d.authMiddleware)
				r.Get("/me", d.handleMe)
				r.Post("/verify-email/request", d.handleRequestVerification)
			})
		})

		// Rutas de evento, protegidas por JWT. Se usan paths /events completos
		// (no Route con "/") para evitar la ambiguedad de barra final de chi.
		r.Group(func(r chi.Router) {
			r.Use(d.authMiddleware)
			r.Post("/events", d.handleCreateEvent)
			r.Get("/events", d.handleQueryEvents)
			r.Get("/events/export", d.handleExportEvents)
			r.Post("/events/import", d.handleImportEvents)
			r.Get("/events/{id}", d.handleGetEvent)
			r.Patch("/events/{id}", d.handleUpdateEvent)
			r.Patch("/events/{id}/status", d.handleChangeStatus)
			r.Delete("/events/{id}", d.handleDeleteEvent)

			// Cuenta del usuario autenticado: vincular/desvincular Google (el
			// usuario ya esta logueado, por eso es seguro) y borrar la cuenta
			// (cascada a todos sus datos).
			r.Post("/account/link-google", d.handleLinkGoogle)
			r.Delete("/account/link-google", d.handleUnlinkGoogle)
			r.Delete("/account", d.handleDeleteAccount)
		})
	})

	return r
}

// claimsCtxKey es la llave privada para inyectar los claims verificados.
type claimsCtxKey struct{}

func withClaims(ctx context.Context, c ports.Claims) context.Context {
	return context.WithValue(ctx, claimsCtxKey{}, c)
}

// userIDFrom lee el sub (identidad autoritativa) inyectado por el middleware.
func userIDFrom(r *http.Request) string {
	if c, ok := r.Context().Value(claimsCtxKey{}).(ports.Claims); ok {
		return c.Subject
	}
	return ""
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, http.StatusNotFound, "event_not_found", "recurso no encontrado")
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "metodo no permitido en esta ruta")
}
