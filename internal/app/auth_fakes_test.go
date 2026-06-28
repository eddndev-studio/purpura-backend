package app

import (
	"context"
	"strings"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// fakeUserRepo es un UserRepository en memoria con unicidad de email (lower) y
// almacen separado de hashes de credencial (espeja la tabla user_credentials).
type fakeUserRepo struct {
	byID    map[string]*domain.User
	byEmail map[string]string // lower(email) -> userID
	bySub   map[string]string // google_sub -> userID
	creds   map[string]string // userID -> passwordHash
}

var _ ports.UserRepository = (*fakeUserRepo)(nil)

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:    map[string]*domain.User{},
		byEmail: map[string]string{},
		bySub:   map[string]string{},
		creds:   map[string]string{},
	}
}

func (r *fakeUserRepo) put(u *domain.User) {
	cp := *u
	r.byID[u.ID] = &cp
	r.byEmail[strings.ToLower(u.Email)] = u.ID
	if u.GoogleSub != nil {
		r.bySub[*u.GoogleSub] = u.ID
	}
}

func (r *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	if _, ok := r.byEmail[strings.ToLower(u.Email)]; ok {
		return domain.ErrEmailTaken
	}
	r.put(u)
	return nil
}

func (r *fakeUserRepo) CreateWithPassword(_ context.Context, u *domain.User, passwordHash string) error {
	if _, ok := r.byEmail[strings.ToLower(u.Email)]; ok {
		return domain.ErrEmailTaken
	}
	r.put(u)
	r.creds[u.ID] = passwordHash
	return nil
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	id, ok := r.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *r.byID[id]
	return &cp, nil
}

func (r *fakeUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *fakeUserRepo) FindByGoogleSub(_ context.Context, sub string) (*domain.User, error) {
	id, ok := r.bySub[sub]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *r.byID[id]
	return &cp, nil
}

func (r *fakeUserRepo) LinkGoogleSub(_ context.Context, userID, sub string) error {
	if owner, ok := r.bySub[sub]; ok && owner != userID {
		return domain.ErrGoogleLinkConflict
	}
	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	if u.GoogleSub != nil {
		delete(r.bySub, *u.GoogleSub)
	}
	s := sub
	u.GoogleSub = &s
	r.bySub[sub] = userID
	return nil
}

func (r *fakeUserRepo) ClearGoogleSub(_ context.Context, userID string) error {
	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	if u.GoogleSub != nil {
		delete(r.bySub, *u.GoogleSub)
		u.GoogleSub = nil
	}
	return nil
}

func (r *fakeUserRepo) SetEmailVerified(_ context.Context, userID string) error {
	u, ok := r.byID[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.EmailVerified = true
	return nil
}

func (r *fakeUserRepo) GetPasswordHash(_ context.Context, userID string) (string, error) {
	h, ok := r.creds[userID]
	if !ok {
		return "", domain.ErrInvalidCredential
	}
	return h, nil
}

func (r *fakeUserRepo) DeleteAccount(_ context.Context, id string) error {
	u, ok := r.byID[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	// Espeja el ON DELETE CASCADE: al borrar el usuario caen su credencial y los
	// indices por email y por google_sub.
	delete(r.byEmail, strings.ToLower(u.Email))
	if u.GoogleSub != nil {
		delete(r.bySub, *u.GoogleSub)
	}
	delete(r.creds, id)
	delete(r.byID, id)
	return nil
}

// fakeTokenService emite un token deterministico y verifica el formato inverso.
type fakeTokenService struct{}

var _ ports.TokenService = fakeTokenService{}

func (fakeTokenService) Issue(_ context.Context, u *domain.User) (ports.IssuedToken, error) {
	return ports.IssuedToken{
		AccessToken: "token-" + u.ID,
		TokenType:   "Bearer",
		ExpiresIn:   86400,
	}, nil
}

func (fakeTokenService) Verify(_ context.Context, accessToken string) (ports.Claims, error) {
	sub := strings.TrimPrefix(accessToken, "token-")
	return ports.Claims{Subject: sub}, nil
}

// fakeGoogleVerifier devuelve una identidad o un error configurables.
type fakeGoogleVerifier struct {
	identity ports.GoogleIdentity
	err      error
}

var _ ports.GoogleVerifier = fakeGoogleVerifier{}

func (g fakeGoogleVerifier) Verify(_ context.Context, _ string) (ports.GoogleIdentity, error) {
	if g.err != nil {
		return ports.GoogleIdentity{}, g.err
	}
	return g.identity, nil
}

// fakeHasher modela un hash reversible ("hash:"+plain); Compare devuelve
// ErrInvalidCredential ante desajuste (contrato del puerto).
type fakeHasher struct{}

var _ ports.PasswordHasher = fakeHasher{}

func (fakeHasher) Hash(_ context.Context, plain string) (string, error) {
	return "hash:" + plain, nil
}

func (fakeHasher) Compare(_ context.Context, hash, plain string) error {
	if hash != "hash:"+plain {
		return domain.ErrInvalidCredential
	}
	return nil
}
