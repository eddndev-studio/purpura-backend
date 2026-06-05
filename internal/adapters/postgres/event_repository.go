package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddndev-studio/purpura-backend/internal/db"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// EventRepository implementa ports.EventRepository. El CRUD estatico usa sqlc;
// la consulta dinamica (filtros opcionales + orden variable + paginacion) se
// construye a mano con pgx, porque sqlc no expresa ORDER BY variable.
type EventRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ ports.EventRepository = (*EventRepository)(nil)

// NewEventRepository construye el repositorio sobre el pool dado.
func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool, q: db.New(pool)}
}

// Create persiste un evento nuevo (id/created/updated ya sellados por el caso de uso).
func (r *EventRepository) Create(ctx context.Context, e *domain.Event) error {
	if _, err := r.q.CreateEvent(ctx, createEventParams(e)); err != nil {
		return fmt.Errorf("postgres: create event: %w", err)
	}
	return nil
}

// FindByID devuelve el evento propio; 0 filas -> ErrEventNotFound.
func (r *EventRepository) FindByID(ctx context.Context, userID, id string) (*domain.Event, error) {
	row, err := r.q.GetEventByID(ctx, db.GetEventByIDParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEventNotFound
		}
		return nil, fmt.Errorf("postgres: get event: %w", err)
	}
	ev := toDomainEvent(row)
	return &ev, nil
}

// Update persiste los campos editables; 0 filas (inexistente/ajeno) -> ErrEventNotFound.
func (r *EventRepository) Update(ctx context.Context, e *domain.Event) error {
	if _, err := r.q.UpdateEvent(ctx, updateEventParams(e)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrEventNotFound
		}
		return fmt.Errorf("postgres: update event: %w", err)
	}
	return nil
}

// Delete elimina el evento propio; 0 filas afectadas -> ErrEventNotFound.
func (r *EventRepository) Delete(ctx context.Context, userID, id string) error {
	n, err := r.q.DeleteEvent(ctx, db.DeleteEventParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("postgres: delete event: %w", err)
	}
	if n == 0 {
		return domain.ErrEventNotFound
	}
	return nil
}

// eventSortColumns es la lista blanca de columnas ordenables. La capa de
// aplicacion ya valida el sort, pero el repo vuelve a comprobar antes de
// interpolar la columna en el ORDER BY (defensa en profundidad; nunca proviene
// del cliente sin pasar por esta lista).
var eventSortColumns = map[string]bool{
	"starts_at":    true,
	"created_at":   true,
	"updated_at":   true,
	"event_type":   true,
	"event_status": true,
}

const eventColumns = "id, user_id, event_type, contact_name, contact_ref, " +
	"location_lat, location_lng, location_label, description, starts_at, " +
	"event_status, reminder_type, created_at, updated_at"

// Query construye el WHERE con los filtros presentes (ventana semiabierta
// [Start,End), type, status), cuenta el total y devuelve la pagina ordenada.
// Las fronteras de fecha llegan ya resueltas a UTC desde la app layer.
func (r *EventRepository) Query(ctx context.Context, q ports.EventQuery) ([]domain.Event, int, error) {
	conds := []string{"user_id = $1"}
	args := []any{q.UserID}

	if q.HasWindow {
		args = append(args, q.Start)
		conds = append(conds, fmt.Sprintf("starts_at >= $%d", len(args)))
		args = append(args, q.End)
		conds = append(conds, fmt.Sprintf("starts_at < $%d", len(args)))
	}
	if q.Type != nil {
		args = append(args, string(*q.Type))
		conds = append(conds, fmt.Sprintf("event_type = $%d", len(args)))
	}
	if q.Status != nil {
		args = append(args, string(*q.Status))
		conds = append(conds, fmt.Sprintf("event_status = $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: count events: %w", err)
	}

	col := q.SortBy
	if !eventSortColumns[col] {
		col = "starts_at"
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	// Orden secundario id ASC para estabilidad de paginacion (04 seccion 6.2).
	sql := "SELECT " + eventColumns + " FROM events WHERE " + where +
		" ORDER BY " + col + " " + dir + ", id ASC"
	if q.Limit > 0 {
		args = append(args, q.Limit)
		sql += fmt.Sprintf(" LIMIT $%d", len(args))
		args = append(args, q.Offset)
		sql += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: query events: %w", err)
	}
	defer rows.Close()

	var out []domain.Event
	for rows.Next() {
		var e db.Event
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.EventType, &e.ContactName, &e.ContactRef,
			&e.LocationLat, &e.LocationLng, &e.LocationLabel, &e.Description,
			&e.StartsAt, &e.EventStatus, &e.ReminderType, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("postgres: scan event: %w", err)
		}
		out = append(out, toDomainEvent(e))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: iterar eventos: %w", err)
	}
	return out, total, nil
}
