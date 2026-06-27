-- name: CreateUser :one
-- id y created_at los provee la aplicacion. Una violacion de
-- users_email_lower_uniq se traduce en ErrEmailTaken.
INSERT INTO users (id, email, nombre, auth_provider, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
-- Lookup case-insensitive via el indice funcional users_email_lower_uniq.
-- 0 filas -> ErrUserNotFound.
SELECT * FROM users
WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: DeleteUser :execrows
-- Borra la cuenta. events y user_credentials referencian users con ON DELETE
-- CASCADE, asi que esta unica sentencia elimina tambien todos los datos del
-- usuario. Devuelve las filas afectadas: 0 -> ErrUserNotFound.
DELETE FROM users
WHERE id = $1;
