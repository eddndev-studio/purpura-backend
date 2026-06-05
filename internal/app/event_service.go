package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// EventService orquesta los casos de uso de eventos. Recibe sus dependencias por
// inyeccion (puertos); no conoce adaptadores concretos.
type EventService struct {
	Events ports.EventRepository
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

// sortColumns mapea el campo ordenable del contrato (camelCase) a la columna
// snake_case que el repositorio usa en ORDER BY (04 seccion 6.2).
var sortColumns = map[string]string{
	"startsAt":    "starts_at",
	"createdAt":   "created_at",
	"updatedAt":   "updated_at",
	"eventType":   "event_type",
	"eventStatus": "event_status",
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
	defaultSort     = "startsAt:asc"
)

// CreateEvent construye el evento (NewEvent fija Status=pendiente y valida las
// invariantes 1-4,7), sella id/created/updated via los puertos y lo persiste. El
// userID proviene SIEMPRE del claim sub; nunca del cuerpo (04 seccion 5.5).
func (s *EventService) CreateEvent(ctx context.Context, userID string, in CreateEventInput) (*domain.Event, error) {
	e, err := domain.NewEvent(userID, in.Type, in.Contact, in.Location, in.Description, in.StartsAt, in.Reminder)
	if err != nil {
		return nil, err
	}
	e.ID = s.IDs.NewID()
	now := s.Clock.Now()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := s.Events.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// GetEvent devuelve el evento si existe y es propio; si no -> ErrEventNotFound
// (404, sin filtrar existencia).
func (s *EventService) GetEvent(ctx context.Context, userID, id string) (*domain.Event, error) {
	return s.Events.FindByID(ctx, userID, id)
}

// UpdateEvent aplica un PATCH parcial: cuerpo vacio -> ErrValidation; carga el
// evento propio (404 si ajeno); revalida via Edit; re-sella UpdatedAt; persiste.
func (s *EventService) UpdateEvent(ctx context.Context, userID, id string, in UpdateEventInput) (*domain.Event, error) {
	if in.Patch.IsEmpty() {
		return nil, fmt.Errorf("%w: el cuerpo no trae ningun campo a actualizar", ErrValidation)
	}
	e, err := s.Events.FindByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := e.Edit(in.Patch); err != nil {
		return nil, err
	}
	e.UpdatedAt = s.Clock.Now()
	if err := s.Events.Update(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ChangeStatus cambia el estatus (transiciones libres) y re-sella UpdatedAt.
func (s *EventService) ChangeStatus(ctx context.Context, userID, id string, status domain.EventStatus) (*domain.Event, error) {
	e, err := s.Events.FindByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := e.ChangeStatus(status); err != nil {
		return nil, err
	}
	e.UpdatedAt = s.Clock.Now()
	if err := s.Events.Update(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// DeleteEvent elimina el evento propio; inexistente/ajeno -> ErrEventNotFound.
func (s *EventService) DeleteEvent(ctx context.Context, userID, id string) error {
	return s.Events.Delete(ctx, userID, id)
}

// QueryEvents resuelve la ventana temporal (query_window), valida paginacion y
// orden, construye el criterio ports.EventQuery y devuelve la pagina + metadatos.
func (s *EventService) QueryEvents(ctx context.Context, userID string, in QueryEventsInput) (QueryEventsResult, error) {
	if err := validateFilters(in.Type, in.Status); err != nil {
		return QueryEventsResult{}, err
	}
	page, pageSize, err := normalizePagination(in.Page, in.PageSize)
	if err != nil {
		return QueryEventsResult{}, err
	}
	sortStr, sortCol, desc, err := resolveSort(in.Sort)
	if err != nil {
		return QueryEventsResult{}, err
	}
	w, err := resolveWindow(in)
	if err != nil {
		return QueryEventsResult{}, err
	}

	q := ports.EventQuery{
		UserID:    userID,
		HasWindow: w.Has,
		Start:     w.Start,
		End:       w.End,
		Type:      in.Type,
		Status:    in.Status,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		SortBy:    sortCol,
		Desc:      desc,
	}
	events, totalItems, err := s.Events.Query(ctx, q)
	if err != nil {
		return QueryEventsResult{}, err
	}
	return QueryEventsResult{
		Events:     events,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: ceilDiv(totalItems, pageSize),
		Sort:       sortStr,
	}, nil
}

// ExportEvents reutiliza ventana/filtros de QueryEvents pero SIN paginar: trae
// todos los eventos del usuario que cumplen el filtro (Limit=0 => sin limite),
// ordenados por startsAt asc. Sirve como documento de respaldo (04 seccion 5.11).
func (s *EventService) ExportEvents(ctx context.Context, userID string, in QueryEventsInput) (ExportResult, error) {
	if err := validateFilters(in.Type, in.Status); err != nil {
		return ExportResult{}, err
	}
	w, err := resolveWindow(in)
	if err != nil {
		return ExportResult{}, err
	}
	q := ports.EventQuery{
		UserID:    userID,
		HasWindow: w.Has,
		Start:     w.Start,
		End:       w.End,
		Type:      in.Type,
		Status:    in.Status,
		Limit:     0, // sin limite: exporta todo el conjunto filtrado
		Offset:    0,
		SortBy:    "starts_at",
		Desc:      false,
	}
	events, _, err := s.Events.Query(ctx, q)
	if err != nil {
		return ExportResult{}, err
	}
	return ExportResult{
		SchemaVersion: "1.0",
		ExportedAt:    s.Clock.Now(),
		UserID:        userID,
		Count:         len(events),
		Events:        events,
	}, nil
}

// validateFilters rechaza valores de filtro type/status fuera del enum. El path
// de LECTURA no invoca ningun constructor/mutador de dominio (a diferencia de
// crear/editar), por lo que necesita su propia guarda: sin ella, un ?type=basura
// produciria un WHERE imposible y un 200 vacio en vez del 422 que exige el
// contrato (04 seccion 4.1/5.7.1). nil = filtro ausente, se omite.
func validateFilters(typ *domain.EventType, status *domain.EventStatus) error {
	if typ != nil && !typ.Valid() {
		return domain.ErrInvalidEventType
	}
	if status != nil && !status.Valid() {
		return domain.ErrInvalidStatus
	}
	return nil
}

// normalizePagination acota page/pageSize: 0 => default; negativo -> ErrValidation;
// pageSize > 100 se acota a 100 (04 seccion 6.1).
func normalizePagination(page, pageSize int) (int, int, error) {
	if page < 0 || pageSize < 0 {
		return 0, 0, fmt.Errorf("%w: page/pageSize no pueden ser negativos", ErrValidation)
	}
	if page == 0 {
		page = 1
	}
	switch {
	case pageSize == 0:
		pageSize = defaultPageSize
	case pageSize > maxPageSize:
		pageSize = maxPageSize
	}
	return page, pageSize, nil
}

// resolveSort valida "campo:direccion" y devuelve el orden efectivo, la columna
// snake_case y la direccion. Vacio => default startsAt:asc. Invalido -> ErrValidation.
func resolveSort(sort string) (effective, column string, desc bool, err error) {
	if sort == "" {
		sort = defaultSort
	}
	field, dir, ok := strings.Cut(sort, ":")
	if !ok {
		return "", "", false, fmt.Errorf("%w: sort debe ser campo:direccion (p.ej. startsAt:asc)", ErrValidation)
	}
	col, ok := sortColumns[field]
	if !ok {
		return "", "", false, fmt.Errorf("%w: campo de orden no soportado: %q", ErrValidation, field)
	}
	switch dir {
	case "asc":
		desc = false
	case "desc":
		desc = true
	default:
		return "", "", false, fmt.Errorf("%w: direccion de orden invalida: %q", ErrValidation, dir)
	}
	return field + ":" + dir, col, desc, nil
}

// ceilDiv = ceil(a/b) con b>0; 0 elementos => 0 paginas.
func ceilDiv(a, b int) int {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
