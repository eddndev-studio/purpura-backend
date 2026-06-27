package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eddndev-studio/purpura-backend/internal/db"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// emailUniqueConstraint es el indice unico funcional de 05 seccion 3; su
// violacion se traduce en ErrEmailTaken.
const emailUniqueConstraint = "users_email_lower_uniq"

// UserRepository implementa ports.UserRepository sobre sqlc + pgx.
type UserRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ ports.UserRepository = (*UserRepository)(nil)

// NewUserRepository construye el repositorio sobre el pool dado.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, q: db.New(pool)}
}

// Create persiste un usuario (sin credencial local). Email duplicado -> ErrEmailTaken.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	if _, err := r.q.CreateUser(ctx, createUserParams(u)); err != nil {
		if isUniqueViolation(err, emailUniqueConstraint) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

// CreateWithPassword inserta usuario y credencial en una sola transaccion: o se
// crean ambos, o ninguno. Email duplicado -> ErrEmailTaken (con rollback).
func (r *UserRepository) CreateWithPassword(ctx context.Context, u *domain.User, passwordHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op si ya se hizo commit

	qtx := r.q.WithTx(tx)
	if _, err := qtx.CreateUser(ctx, createUserParams(u)); err != nil {
		if isUniqueViolation(err, emailUniqueConstraint) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("postgres: create user (tx): %w", err)
	}
	if err := qtx.CreateCredential(ctx, db.CreateCredentialParams{UserID: u.ID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("postgres: create credential (tx): %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

// FindByEmail busca por email (lower); 0 filas -> ErrUserNotFound.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres: get user by email: %w", err)
	}
	u := toDomainUser(row)
	return &u, nil
}

// FindByID busca por id; 0 filas -> ErrUserNotFound.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres: get user by id: %w", err)
	}
	u := toDomainUser(row)
	return &u, nil
}

// GetPasswordHash lee el hash de credencial; ausencia -> ErrInvalidCredential
// (no distingue "sin cuenta" de "sin password", para no filtrar existencia).
func (r *UserRepository) GetPasswordHash(ctx context.Context, userID string) (string, error) {
	hash, err := r.q.GetPasswordHashByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrInvalidCredential
		}
		return "", fmt.Errorf("postgres: get password hash: %w", err)
	}
	return hash, nil
}

// DeleteAccount elimina al usuario por id; el resto de sus datos cae por
// ON DELETE CASCADE (events, user_credentials). 0 filas -> ErrUserNotFound.
func (r *UserRepository) DeleteAccount(ctx context.Context, id string) error {
	n, err := r.q.DeleteUser(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: delete user: %w", err)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// isUniqueViolation indica si err es una violacion de unicidad (SQLSTATE 23505)
// de la restriccion dada (vacio = cualquiera).
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
	}
	return false
}
