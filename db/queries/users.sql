-- name: CreateUser :one
-- id y created_at los provee la aplicacion. google_sub es NULL salvo en cuentas
-- nacidas de Google (se sella con el sub del idToken). Una violacion de
-- users_email_lower_uniq se traduce en ErrEmailTaken.
INSERT INTO users (id, email, nombre, auth_provider, google_sub, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByEmail :one
-- Lookup case-insensitive via el indice funcional users_email_lower_uniq.
-- 0 filas -> ErrUserNotFound.
SELECT * FROM users
WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByGoogleSub :one
-- Lookup por el sub inmutable de Google (la llave de vinculacion, no el email).
-- 0 filas -> ErrUserNotFound.
SELECT * FROM users
WHERE google_sub = $1;

-- name: LinkGoogleSub :execrows
-- Adjunta el sub de Google a la cuenta. Una violacion de unicidad (el sub ya
-- esta en otra cuenta) la traduce el repo a ErrGoogleLinkConflict.
-- 0 filas -> ErrUserNotFound.
UPDATE users SET google_sub = $2
WHERE id = $1;

-- name: ClearGoogleSub :execrows
-- Desvincula Google de la cuenta (google_sub = NULL). 0 filas -> ErrUserNotFound.
UPDATE users SET google_sub = NULL
WHERE id = $1;

-- name: DeleteUser :execrows
-- Borra la cuenta. events y user_credentials referencian users con ON DELETE
-- CASCADE, asi que esta unica sentencia elimina tambien todos los datos del
-- usuario. Devuelve las filas afectadas: 0 -> ErrUserNotFound.
DELETE FROM users
WHERE id = $1;
