// Package postgres implementa los puertos EventRepository y UserRepository sobre
// el paquete generado por sqlc (internal/db) y un pool pgx. Es un adaptador
// driven: lo llaman los casos de uso a traves de los puertos. Traduce modelos
// sqlc <-> entidades de dominio (mapping.go) y nunca expone tipos de db hacia
// afuera.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool construye el pool pgx desde la URL de conexion y verifica la
// conectividad con un ping. El composition root (cmd/api) lo cierra al apagar.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("postgres: no se pudo crear el pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping inicial fallo: %w", err)
	}
	return pool, nil
}
