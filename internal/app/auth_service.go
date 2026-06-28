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
// la verificacion falla -> ErrUnauthorized (401). Reconcilia por el SUB inmutable
// (no por email, que se recicla): (1) si ya hay cuenta con ese sub -> login; (2)
// si no, por email: no existe -> crea cuenta google sellada con el sub; cuenta
// google legacy sin sub -> retro-rellena el sub y entra; en otro caso (cuenta
// password sin vinculo, u otra cuenta con otro Google) -> ErrEmailTaken (409),
// porque NO se mergea por email: para usar Google con una cuenta de contrasena
// hay que vincular desde Ajustes (estando logueado).
func (s *AuthService) AuthenticateWithGoogle(ctx context.Context, idToken string) (AuthResult, error) {
	identity, err := s.Google.Verify(ctx, idToken)
	if err != nil {
		return AuthResult{}, fmt.Errorf("%w: idToken de Google no verificable", ErrUnauthorized)
	}

	// 1) Llave estable: el sub. Si ya existe una cuenta con ese sub -> login.
	if u, ferr := s.Users.FindByGoogleSub(ctx, identity.Sub); ferr == nil {
		return s.issue(ctx, u)
	} else if !errors.Is(ferr, domain.ErrUserNotFound) {
		return AuthResult{}, ferr
	}

	// 2) Sin cuenta por sub: reconciliar por email.
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	u, err := s.Users.FindByEmail(ctx, email)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		// No existe: crear cuenta de ORIGEN Google, sellada con su sub.
		nu, nerr := domain.NewUser(identity.Email, identity.Nombre, domain.AuthGoogle)
		if nerr != nil {
			return AuthResult{}, nerr
		}
		nu.ID = s.IDs.NewID()
		nu.CreatedAt = s.Clock.Now()
		sub := identity.Sub
		nu.GoogleSub = &sub
		if cerr := s.Users.Create(ctx, nu); cerr != nil {
			return AuthResult{}, cerr
		}
		return s.issue(ctx, nu)
	case err != nil:
		return AuthResult{}, err
	case u.GoogleSub == nil && u.AuthProvider == domain.AuthGoogle:
		// Cuenta Google legacy (creada por email antes del llaveo por sub):
		// retro-rellenar el sub (misma identidad) y entrar.
		if lerr := s.Users.LinkGoogleSub(ctx, u.ID, identity.Sub); lerr != nil {
			return AuthResult{}, lerr
		}
		return s.issue(ctx, u)
	default:
		// Email de una cuenta password sin vinculo, o con OTRO Google adjunto.
		return AuthResult{}, domain.ErrEmailTaken
	}
}

// LinkGoogle adjunta la identidad Google del idToken a la cuenta autenticada
// (userID viene del sub del JWT). Es seguro SIN verificar el email: el usuario ya
// probo ser dueno de la cuenta (esta logueado) y de la cuenta Google (idToken
// valido con su sub). Idempotente si ya esta vinculada al MISMO sub; si la cuenta
// ya tiene otro Google, o el sub pertenece a otra cuenta -> ErrGoogleLinkConflict.
// Devuelve el usuario actualizado (sin emitir token: el usuario ya esta logueado).
func (s *AuthService) LinkGoogle(ctx context.Context, userID, idToken string) (*domain.User, error) {
	identity, err := s.Google.Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("%w: idToken de Google no verificable", ErrUnauthorized)
	}
	u, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.GoogleSub != nil {
		if *u.GoogleSub == identity.Sub {
			return u, nil // idempotente: ya vinculada a este Google
		}
		return nil, domain.ErrGoogleLinkConflict // ya tiene otro Google adjunto
	}
	// LinkGoogleSub mapea la violacion de unicidad (sub en otra cuenta) a
	// ErrGoogleLinkConflict de forma atomica.
	if lerr := s.Users.LinkGoogleSub(ctx, userID, identity.Sub); lerr != nil {
		return nil, lerr
	}
	sub := identity.Sub
	u.GoogleSub = &sub
	return u, nil
}

// UnlinkGoogle desvincula Google de la cuenta autenticada. Solo se permite si la
// cuenta conserva otro metodo de acceso (credencial de contrasena): si no,
// quedaria sin forma de iniciar sesion (y ademas se re-vincularia sola en el
// siguiente login con Google) -> ErrCannotUnlinkGoogle. Idempotente si ya estaba
// desvinculada. Devuelve el usuario actualizado.
func (s *AuthService) UnlinkGoogle(ctx context.Context, userID string) (*domain.User, error) {
	u, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.GoogleSub == nil {
		return u, nil // idempotente: ya desvinculada
	}
	if _, perr := s.Users.GetPasswordHash(ctx, userID); perr != nil {
		if errors.Is(perr, domain.ErrInvalidCredential) {
			return nil, domain.ErrCannotUnlinkGoogle
		}
		return nil, perr
	}
	if cerr := s.Users.ClearGoogleSub(ctx, userID); cerr != nil {
		return nil, cerr
	}
	u.GoogleSub = nil
	return u, nil
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
