package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddndev-studio/purpura-backend/internal/db"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// VerificationTokenRepository implementa ports.VerificationTokenRepository sobre
// sqlc + pgx. Solo persiste el hash del token; el valor crudo nunca toca la BD.
type VerificationTokenRepository struct {
	q *db.Queries
}

var _ ports.VerificationTokenRepository = (*VerificationTokenRepository)(nil)

// NewVerificationTokenRepository construye el repositorio sobre el pool dado.
func NewVerificationTokenRepository(pool *pgxpool.Pool) *VerificationTokenRepository {
	return &VerificationTokenRepository{q: db.New(pool)}
}

// Create persiste un token nuevo (sin usar).
func (r *VerificationTokenRepository) Create(ctx context.Context, t *ports.VerificationToken) error {
	if err := r.q.CreateVerificationToken(ctx, createVerificationTokenParams(t)); err != nil {
		return fmt.Errorf("postgres: create verification token: %w", err)
	}
	return nil
}

// FindByHash busca por el hash; 0 filas -> ErrInvalidVerificationToken (no
// distingue inexistente de usado: ninguno es utilizable).
func (r *VerificationTokenRepository) FindByHash(ctx context.Context, hash string) (*ports.VerificationToken, error) {
	row, err := r.q.GetVerificationTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidVerificationToken
		}
		return nil, fmt.Errorf("postgres: get verification token by hash: %w", err)
	}
	t := toDomainVerificationToken(row)
	return &t, nil
}

// MarkUsed marca el token como usado de un solo uso (UPDATE ... WHERE used_at IS
// NULL). true si lo marco; false si ya estaba usado o no existe.
func (r *VerificationTokenRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) (bool, error) {
	n, err := r.q.MarkVerificationTokenUsed(ctx, db.MarkVerificationTokenUsedParams{
		ID:     id,
		UsedAt: pgtype.Timestamptz{Time: usedAt, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("postgres: mark verification token used: %w", err)
	}
	return n > 0, nil
}
