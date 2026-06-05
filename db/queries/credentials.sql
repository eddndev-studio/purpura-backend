-- name: CreateCredential :exec
-- Inserta el hash de una cuenta local. Corre en la MISMA transaccion (pgx) que
-- el INSERT en users de register (04 seccion 5.2), para garantizar atomicidad.
INSERT INTO user_credentials (user_id, password_hash)
VALUES ($1, $2);

-- name: GetPasswordHashByUserID :one
-- Lee el hash para verificar el login (04 seccion 5.3). La ausencia de fila
-- (cuenta google, o usuario inexistente) -> domain.ErrInvalidCredential.
SELECT password_hash
FROM user_credentials
WHERE user_id = $1;

-- name: UpdatePasswordHash :exec
-- Actualiza el hash (cambio de contrasena). La aplicacion fija updated_at.
UPDATE user_credentials
SET password_hash = $2,
    updated_at    = now()
WHERE user_id = $1;
