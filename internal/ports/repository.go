// Package ports define las interfaces (puertos) del nucleo hacia el exterior.
// Los casos de uso (internal/app) dependen de estas interfaces; los adaptadores
// driven (postgres, auth, sys) las implementan. La direccion de dependencia
// apunta siempre al dominio: los puertos usan tipos de dominio, no DTOs.
package ports

import (
	"context"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// EventQuery es el criterio de consulta ya RESUELTO que el repositorio traduce
// a SQL. La traduccion de mode/tz a la ventana [Start, End) ocurre en la capa de
// aplicacion (query_window); el repositorio nunca hace aritmetica de calendario.
type EventQuery struct {
	UserID string

	// Ventana temporal semiabierta [Start, End) sobre starts_at, en UTC.
	// Si HasWindow es false, no se filtra por tiempo (modo "listar todos").
	HasWindow bool
	Start     time.Time // inclusive
	End       time.Time // exclusive

	Type   *domain.EventType   // filtro opcional por tipo
	Status *domain.EventStatus // filtro opcional por estatus

	// Paginacion y orden ya validados/normalizados por la capa de aplicacion.
	Limit  int    // pageSize efectivo (1..100)
	Offset int    // (page-1)*pageSize
	SortBy string // starts_at | created_at | updated_at | event_type | event_status
	Desc   bool   // direccion del orden
}

// EventRepository persiste y consulta eventos. Todas las lecturas y mutaciones
// estan acotadas al propietario (userID): el scoping por usuario es estructural.
// El acceso a un evento ajeno devuelve domain.ErrEventNotFound (nunca filtra
// la existencia).
type EventRepository interface {
	// Create persiste un evento nuevo (ya con ID, CreatedAt y UpdatedAt
	// asignados por el caso de uso).
	Create(ctx context.Context, e *domain.Event) error

	// FindByID devuelve el evento si existe y pertenece a userID; en otro caso
	// domain.ErrEventNotFound.
	FindByID(ctx context.Context, userID, id string) (*domain.Event, error)

	// Update persiste los campos editables de un evento existente del usuario.
	// Si no existe o es ajeno: domain.ErrEventNotFound.
	Update(ctx context.Context, e *domain.Event) error

	// Delete elimina el evento del usuario. Si no existe o es ajeno:
	// domain.ErrEventNotFound.
	Delete(ctx context.Context, userID, id string) error

	// Query devuelve la pagina de eventos que cumplen el criterio y el total de
	// elementos que lo cumplen (sin paginar), para calcular totalPages.
	Query(ctx context.Context, q EventQuery) (events []domain.Event, totalItems int, err error)
}

// UserRepository persiste y consulta usuarios y sus credenciales locales.
type UserRepository interface {
	// Create persiste un usuario nuevo (ya con ID y CreatedAt asignados).
	// Si el email (lower) ya existe: domain.ErrEmailTaken.
	Create(ctx context.Context, u *domain.User) error

	// CreateWithPassword crea el usuario y guarda su hash de credencial en la
	// misma transaccion. El auth_provider del usuario debe ser AuthPassword.
	// Si el email ya existe: domain.ErrEmailTaken.
	CreateWithPassword(ctx context.Context, u *domain.User, passwordHash string) error

	// FindByEmail busca por email normalizado (lower). Si no existe:
	// domain.ErrUserNotFound.
	FindByEmail(ctx context.Context, email string) (*domain.User, error)

	// FindByID busca por id. Si no existe: domain.ErrUserNotFound.
	FindByID(ctx context.Context, id string) (*domain.User, error)

	// FindByGoogleSub busca por el sub inmutable de Google (llave de
	// vinculacion). Si no existe ninguna cuenta con ese sub: domain.ErrUserNotFound.
	FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error)

	// LinkGoogleSub adjunta el sub de Google a la cuenta. Si el sub ya esta en
	// otra cuenta: domain.ErrGoogleLinkConflict. Si el usuario no existe:
	// domain.ErrUserNotFound.
	LinkGoogleSub(ctx context.Context, userID, sub string) error

	// ClearGoogleSub desvincula Google de la cuenta (google_sub = NULL). Si el
	// usuario no existe: domain.ErrUserNotFound.
	ClearGoogleSub(ctx context.Context, userID string) error

	// GetPasswordHash devuelve el hash de credencial del usuario (cuenta
	// password). Si el usuario no tiene credencial local:
	// domain.ErrInvalidCredential (no distingue "no existe" de "sin password").
	GetPasswordHash(ctx context.Context, userID string) (string, error)

	// DeleteAccount elimina al usuario y, por ON DELETE CASCADE, todos sus datos
	// (eventos y credenciales). Si no existe: domain.ErrUserNotFound.
	DeleteAccount(ctx context.Context, id string) error
}
