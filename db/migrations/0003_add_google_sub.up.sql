-- google_sub es la LLAVE de vinculacion con Google: el 'sub' inmutable del
-- idToken (no el email, que se recicla). Nullable y UNIQUE: una cuenta puede no
-- tener Google adjunto (NULL; Postgres permite multiples NULL en un indice
-- UNIQUE, por eso no hace falta un indice parcial), y un mismo sub no puede estar
-- en dos cuentas. Es ortogonal a auth_provider (el proveedor de ORIGEN, que no
-- cambia al vincular): si google_sub != NULL, la cuenta entra tambien por Google.
ALTER TABLE users ADD COLUMN google_sub text UNIQUE;
