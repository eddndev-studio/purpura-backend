package app

import (
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// Los DTOs de entrada/salida son structs Go planos, SIN etiquetas json: son
// independientes del transporte. El adaptador HTTP traduce JSON camelCase <->
// estos DTOs (04 seccion 2). Los campos de enumeracion ya viajan como tipos de
// dominio: el codec solo castea la cadena; la VALIDACION del valor la hace el
// dominio (NewEvent/Edit/ChangeStatus), de modo que un enum invalido produce el
// error de dominio correspondiente (422), no un error de transporte.

// CreateEventInput es la entrada de CreateEvent. NO incluye userId (proviene del
// claim sub) ni eventStatus (el estatus inicial siempre es pendiente; 04 5.5).
type CreateEventInput struct {
	Type        domain.EventType
	Contact     domain.Contact
	Location    domain.Location
	Description string
	StartsAt    time.Time
	Reminder    domain.Reminder
}

// UpdateEventInput es la entrada de UpdateEvent (PATCH parcial). El codec
// construye el EventPatch solo con las claves presentes en el cuerpo (punteros);
// un cuerpo sin campos -> ErrValidation (422).
type UpdateEventInput struct {
	Patch domain.EventPatch
}

// QueryEventsInput es la consulta ya decodificada del query string. Las fechas
// date-only viajan como cadenas YYYY-MM-DD; query_window las parsea y construye
// la ventana en TZ. Type/Status son punteros opcionales (filtro). Page/PageSize/
// Sort se validan y normalizan en QueryEvents.
type QueryEventsInput struct {
	Mode  string // "" | por_dia | por_rango | por_mes | por_anio
	Date  string // YYYY-MM-DD (por_dia)
	From  string // YYYY-MM-DD (por_rango)
	To    string // YYYY-MM-DD (por_rango)
	Year  int    // por_mes / por_anio
	Month int    // por_mes (1..12)
	TZ    string // nombre IANA; "" => UTC

	Type   *domain.EventType
	Status *domain.EventStatus

	Page     int
	PageSize int
	Sort     string // "campo:direccion", p.ej. startsAt:asc
}

// QueryEventsResult es la salida paginada de QueryEvents. El handler la envuelve
// en { data, pagination } (04 seccion 5.7.4).
type QueryEventsResult struct {
	Events     []domain.Event
	Page       int
	PageSize   int
	TotalItems int
	TotalPages int
	Sort       string // orden efectivo aplicado, p.ej. startsAt:asc
}

// ExportResult es el documento de respaldo (04 seccion 5.11). El handler lo
// serializa y anade Content-Disposition.
type ExportResult struct {
	SchemaVersion string
	ExportedAt    time.Time
	UserID        string
	Count         int
	Events        []domain.Event
}

// ImportEventInput es un evento del documento de respaldo (forma plana). A
// diferencia de CreateEventInput, ID es opcional y Status SI se respeta del
// payload (nil = ausente -> queda pendiente como lo fija NewEvent).
type ImportEventInput struct {
	ID          string
	Type        domain.EventType
	Contact     domain.Contact
	Location    domain.Location
	Description string
	StartsAt    time.Time
	Status      *domain.EventStatus
	Reminder    domain.Reminder
}

// ImportInput es la entrada de ImportEvents. Mode "" se trata como partial.
type ImportInput struct {
	Mode   string // "partial" (default) | "atomic"
	Events []ImportEventInput
}

// ImportSummary es el resultado de ImportEvents (04 seccion 5.12). El handler lo
// serializa como { imported, updated, skipped, failed, errors }.
type ImportSummary struct {
	Imported int
	Updated  int
	Skipped  int
	Failed   int
	Errors   []ImportItemError
}

// ImportItemError describe el fallo de un evento del payload por su indice.
type ImportItemError struct {
	Index  int
	Code   string
	Detail string
}
