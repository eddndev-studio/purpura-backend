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
