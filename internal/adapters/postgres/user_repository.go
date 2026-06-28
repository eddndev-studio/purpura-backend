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

// googleSubUniqueConstraint es la restriccion UNIQUE de users.google_sub
// (generada por Postgres al declarar la columna). Su violacion -> ErrGoogleLinkConflict.
const googleSubUniqueConstraint = "users_google_sub_key"

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
		// Cuenta google sellada con un sub que otra peticion concurrente ya creo:
		// se traduce a ErrGoogleLinkConflict (no a un 500), igual que LinkGoogleSub.
		if isUniqueViolation(err, googleSubUniqueConstraint) {
			return domain.ErrGoogleLinkConflict
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

// FindByGoogleSub busca por el sub de Google; 0 filas -> ErrUserNotFound.
func (r *UserRepository) FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	row, err := r.q.GetUserByGoogleSub(ctx, &sub)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres: get user by google sub: %w", err)
	}
	u := toDomainUser(row)
	return &u, nil
}

// LinkGoogleSub adjunta el sub de Google a la cuenta. Sub ya en otra cuenta
// (violacion de unicidad) -> ErrGoogleLinkConflict; 0 filas -> ErrUserNotFound.
func (r *UserRepository) LinkGoogleSub(ctx context.Context, userID, sub string) error {
	n, err := r.q.LinkGoogleSub(ctx, db.LinkGoogleSubParams{ID: userID, GoogleSub: &sub})
	if err != nil {
		if isUniqueViolation(err, googleSubUniqueConstraint) {
			return domain.ErrGoogleLinkConflict
		}
		return fmt.Errorf("postgres: link google sub: %w", err)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// ClearGoogleSub desvincula Google (google_sub = NULL); 0 filas -> ErrUserNotFound.
func (r *UserRepository) ClearGoogleSub(ctx context.Context, userID string) error {
	n, err := r.q.ClearGoogleSub(ctx, userID)
	if err != nil {
		return fmt.Errorf("postgres: clear google sub: %w", err)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// SetEmailVerified marca el correo del usuario como verificado (idempotente).
// 0 filas -> ErrUserNotFound.
func (r *UserRepository) SetEmailVerified(ctx context.Context, userID string) error {
	n, err := r.q.SetEmailVerified(ctx, userID)
	if err != nil {
		return fmt.Errorf("postgres: set email verified: %w", err)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
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
