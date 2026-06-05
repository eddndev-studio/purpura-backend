-- name: CreateEvent :one
-- El id, created_at y updated_at los provee la aplicacion (sellado app-layer).
INSERT INTO events (
    id, user_id, event_type, contact_name, contact_ref,
    location_lat, location_lng, location_label, description,
    starts_at, event_status, reminder_type, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetEventByID :one
-- Scoping estructural por user_id: 0 filas -> ErrEventNotFound (no filtra ajeno).
SELECT * FROM events
WHERE id = $1 AND user_id = $2;

-- name: UpdateEvent :one
-- updated_at lo re-sella la aplicacion (05 seccion 4.1). Scoping por user_id.
UPDATE events
SET event_type     = $3,
    contact_name   = $4,
    contact_ref    = $5,
    location_lat   = $6,
    location_lng   = $7,
    location_label = $8,
    description    = $9,
    starts_at      = $10,
    event_status   = $11,
    reminder_type  = $12,
    updated_at     = $13
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteEvent :execrows
-- Devuelve filas afectadas: 0 -> ErrEventNotFound.
DELETE FROM events
WHERE id = $1 AND user_id = $2;
