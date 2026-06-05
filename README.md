# purpura-backend

API REST para Purpura, el organizador de eventos. Escrita en Go con arquitectura
hexagonal (ports and adapters) y desarrollada con TDD.

## Stack

- Go 1.26
- Router: chi
- Acceso a datos: sqlc sobre PostgreSQL
- Auth: JWT propio + Google Sign-In
- Migraciones: golang-migrate

## Estructura

```
cmd/api/            punto de entrada del servidor HTTP
internal/
  domain/           entidades y reglas de negocio (nucleo, sin dependencias)
  app/              casos de uso (orquestan el dominio via puertos)
  ports/            interfaces (repositorios, servicios externos)
  adapters/
    http/           handlers chi y middleware
    postgres/       repositorios generados con sqlc
    auth/           emision y verificacion de tokens
db/
  migrations/       migraciones SQL
  queries/          consultas para sqlc
```

## Desarrollo

```bash
make db-up      # levanta PostgreSQL en docker
make migrate    # aplica migraciones
make test       # corre la suite de tests
make run        # arranca el servidor
```

## Convenciones

- Archivos < 400 LOC, modulares.
- Commits granulares con conventional commits, en ASCII.
