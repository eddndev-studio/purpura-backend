CREATE TABLE users (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    nombre        text        NOT NULL,
    auth_provider text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_not_empty     CHECK (char_length(btrim(email)) > 0),
    CONSTRAINT users_nombre_not_empty    CHECK (char_length(btrim(nombre)) > 0),
    CONSTRAINT users_auth_provider_valid CHECK (auth_provider IN ('google', 'password'))
);

CREATE UNIQUE INDEX users_email_lower_uniq ON users (lower(email));

CREATE TABLE events (
    id             uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid             NOT NULL,
    event_type     text             NOT NULL,
    contact_name   text             NOT NULL DEFAULT '',
    contact_ref    text             NOT NULL DEFAULT '',
    location_lat   double precision NOT NULL,
    location_lng   double precision NOT NULL,
    location_label text             NOT NULL DEFAULT '',
    description    text             NOT NULL,
    starts_at      timestamptz      NOT NULL,
    event_status   text             NOT NULL DEFAULT 'pendiente',
    reminder_type  text             NOT NULL,
    created_at     timestamptz      NOT NULL DEFAULT now(),
    updated_at     timestamptz      NOT NULL DEFAULT now(),
    CONSTRAINT events_user_id_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT events_event_type_valid    CHECK (event_type IN ('cita', 'junta', 'entrega_proyecto', 'examen', 'otros')),
    CONSTRAINT events_event_status_valid  CHECK (event_status IN ('pendiente', 'realizado', 'aplazado')),
    CONSTRAINT events_reminder_type_valid CHECK (reminder_type IN ('none', 'at_time', 'ten_minutes_before', 'one_day_before')),
    CONSTRAINT events_description_not_empty CHECK (char_length(btrim(description)) > 0),
    CONSTRAINT events_location_lat_range    CHECK (location_lat >= -90  AND location_lat <= 90),
    CONSTRAINT events_location_lng_range    CHECK (location_lng >= -180 AND location_lng <= 180)
);

CREATE INDEX events_user_starts_at_idx        ON events (user_id, starts_at);
CREATE INDEX events_user_type_starts_at_idx   ON events (user_id, event_type, starts_at);
CREATE INDEX events_user_status_starts_at_idx ON events (user_id, event_status, starts_at);
