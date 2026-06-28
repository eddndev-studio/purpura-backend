-- name: CreateVerificationToken :exec
-- id, token_hash, expires_at y created_at los provee la aplicacion. Solo se
-- guarda el hash; el token crudo viaja en el correo y no se persiste.
INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetVerificationTokenByHash :one
-- Busca por el hash del token presentado. 0 filas -> ErrInvalidVerificationToken.
SELECT * FROM email_verification_tokens
WHERE token_hash = $1;

-- name: MarkVerificationTokenUsed :execrows
-- Marca el token como usado de forma atomica y de UN SOLO USO: la condicion
-- used_at IS NULL hace que dos confirmaciones concurrentes solo una afecte filas
-- (la otra ve 0 filas -> token ya usado). 0 filas -> ya usado (o id inexistente).
UPDATE email_verification_tokens SET used_at = $2
WHERE id = $1 AND used_at IS NULL;
