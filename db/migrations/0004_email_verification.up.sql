-- Verificacion de correo (Fase 2). El gate es SUAVE: email_verified nunca bloquea
-- el login; la app solo lo usa para un aviso ("verifica tu correo"). Backfill: SOLO
-- las cuentas de ORIGEN Google nacen verificadas, porque su correo en ficha ES el
-- del idToken de Google que AuthenticateWithGoogle exigio con email_verified=true.
-- OJO: una cuenta password que vinculo Google NO se marca: vincular prueba el sub
-- (la identidad Google), no que el correo en ficha sea suyo; ese correo sigue sin
-- verificar y debe pasar por el flujo normal de verificacion.
ALTER TABLE users ADD COLUMN email_verified boolean NOT NULL DEFAULT false;

UPDATE users SET email_verified = true
WHERE auth_provider = 'google';

-- Tokens de verificacion de correo: de un solo uso y con expiracion. Solo se
-- guarda el HASH del token (sha256), nunca el valor crudo; el crudo viaja en el
-- enlace del correo y no debe poder reconstruirse desde la BD.
CREATE TABLE email_verification_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL,
    token_hash text        NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_verification_tokens_user_fk
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX email_verification_tokens_hash_uniq ON email_verification_tokens (token_hash);
CREATE INDEX email_verification_tokens_user_idx ON email_verification_tokens (user_id);
