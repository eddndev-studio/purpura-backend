CREATE TABLE user_credentials (
    user_id       uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_credentials_password_hash_not_empty CHECK (char_length(password_hash) > 0)
);
