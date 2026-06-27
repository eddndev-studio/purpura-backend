package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// Politica de longitud de contrasena. El minimo es del contrato (04 seccion
// 5.2). El maximo es el limite duro de bcrypt (72 bytes): sin esta guarda, una
// contrasena mas larga haria que Hash devolviera un error de infraestructura que
// el cliente veria como 500; con ella se rechaza como 422 validation_failed.
const (
	minPasswordLen = 8
	maxPasswordLen = 72
)

// AuthService orquesta el registro, login y Google Sign-In. Emite el JWT propio.
type AuthService struct {
	Users  ports.UserRepository
	Tokens ports.TokenService
	Google ports.GoogleVerifier
	Hasher ports.PasswordHasher
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

// Register crea una cuenta local (authProvider=password) y devuelve un JWT.
// NewUser normaliza/valida email y nombre (errores -> validation_failed). La
// politica de password vive en la capa de aplicacion. Email duplicado ->
// ErrEmailTaken (409). User + credencial se crean en la misma transaccion.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
	u, err := domain.NewUser(in.Email, in.Nombre, domain.AuthPassword)
	if err != nil {
		return AuthResult{}, err
	}
	if len(in.Password) < minPasswordLen {
		return AuthResult{}, fmt.Errorf("%w: la contrasena debe tener al menos %d caracteres", ErrValidation, minPasswordLen)
	}
	if len(in.Password) > maxPasswordLen {
		return AuthResult{}, fmt.Errorf("%w: la contrasena no puede exceder %d bytes", ErrValidation, maxPasswordLen)
	}
	u.ID = s.IDs.NewID()
	u.CreatedAt = s.Clock.Now()

	hash, err := s.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.Users.CreateWithPassword(ctx, u, hash); err != nil {
		return AuthResult{}, err
	}
	return s.issue(ctx, u)
}

// Login valida credenciales locales. Para NO filtrar la existencia de cuentas,
// cualquier fallo de credencial (usuario inexistente, sin credencial local, o
// hash que no coincide) se colapsa a ErrInvalidCredential (401). Los fallos de
// infraestructura se propagan (500).
func (s *AuthService) Login(ctx context.Context, in LoginInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	u, err := s.Users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return AuthResult{}, domain.ErrInvalidCredential
		}
		return AuthResult{}, err
	}
	hash, err := s.Users.GetPasswordHash(ctx, u.ID)
	if err != nil {
		// ErrInvalidCredential (sin credencial local) ya no filtra existencia.
		return AuthResult{}, err
	}
	if err := s.Hasher.Compare(ctx, hash, in.Password); err != nil {
		// Compare devuelve ErrInvalidCredential en desajuste; otros son infra.
		return AuthResult{}, err
	}
	return s.issue(ctx, u)
}

// AuthenticateWithGoogle intercambia un idToken de Google por un JWT propio. Si
// la verificacion falla -> ErrUnauthorized (401). Reconcilia por email: crea la
// cuenta google si no existe, la reutiliza si ya es google, y rechaza con
// ErrEmailTaken (409) si el email pertenece a una cuenta password.
func (s *AuthService) AuthenticateWithGoogle(ctx context.Context, idToken string) (AuthResult, error) {
	identity, err := s.Google.Verify(ctx, idToken)
	if err != nil {
		return AuthResult{}, fmt.Errorf("%w: idToken de Google no verificable", ErrUnauthorized)
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))

	u, err := s.Users.FindByEmail(ctx, email)
	switch {
	case err == nil:
		if u.AuthProvider != domain.AuthGoogle {
			return AuthResult{}, domain.ErrEmailTaken
		}
		// Cuenta google existente: reutilizar.
	case errors.Is(err, domain.ErrUserNotFound):
		nu, nerr := domain.NewUser(identity.Email, identity.Nombre, domain.AuthGoogle)
		if nerr != nil {
			return AuthResult{}, nerr
		}
		nu.ID = s.IDs.NewID()
		nu.CreatedAt = s.Clock.Now()
		if cerr := s.Users.Create(ctx, nu); cerr != nil {
			return AuthResult{}, cerr
		}
		u = nu
	default:
		return AuthResult{}, err
	}
	return s.issue(ctx, u)
}

// DeleteAccount elimina permanentemente la cuenta del usuario y, por cascada en
// la BD, todos sus datos (eventos y credencial). userID viene del sub del JWT
// (identidad autoritativa), asi que solo se borra a si mismo. Si la cuenta ya no
// existe -> domain.ErrUserNotFound (404). Nota: el JWT es stateless; un token ya
// emitido sigue siendo criptograficamente valido hasta expirar, pero toda
// operacion de datos posterior fallara porque el usuario dejo de existir.
func (s *AuthService) DeleteAccount(ctx context.Context, userID string) error {
	return s.Users.DeleteAccount(ctx, userID)
}

// issue emite el access token para el usuario y arma el AuthResult.
func (s *AuthService) issue(ctx context.Context, u *domain.User) (AuthResult, error) {
	tok, err := s.Tokens.Issue(ctx, u)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: tok, User: u}, nil
}
