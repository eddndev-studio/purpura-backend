package app

import (
	"context"
	"errors"
	"testing"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

func newAuthSvc() (*AuthService, *fakeUserRepo) {
	repo := newFakeUserRepo()
	return &AuthService{
		Users:  repo,
		Tokens: fakeTokenService{},
		Google: fakeGoogleVerifier{},
		Hasher: fakeHasher{},
		Clock:  fixedClock{t: fixedNow},
		IDs:    &seqIDGen{},
	}, repo
}

func TestRegister_Success(t *testing.T) {
	svc, repo := newAuthSvc()
	res, err := svc.Register(context.Background(), RegisterInput{
		Email: "Ana@Example.com", Nombre: " Ana ", Password: "S3guroPurpura!",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.User.ID != "id-1" || res.User.Email != "ana@example.com" || res.User.Nombre != "Ana" {
		t.Errorf("usuario normalizado mal: %+v", res.User)
	}
	if res.User.AuthProvider != domain.AuthPassword || !res.User.CreatedAt.Equal(fixedNow) {
		t.Errorf("provider/createdAt mal: %+v", res.User)
	}
	if res.Token.AccessToken != "token-id-1" || res.Token.TokenType != "Bearer" || res.Token.ExpiresIn != 86400 {
		t.Errorf("token mal: %+v", res.Token)
	}
	hash, err := repo.GetPasswordHash(context.Background(), "id-1")
	if err != nil || hash != "hash:S3guroPurpura!" {
		t.Errorf("credencial no persistida: %q err=%v", hash, err)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc, _ := newAuthSvc()
	_, err := svc.Register(context.Background(), RegisterInput{Email: "a@x.com", Nombre: "A", Password: "1234567"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("password corta -> ErrValidation, obtuve %v", err)
	}
}

func TestRegister_TooLongPasswordIsValidation(t *testing.T) {
	svc, _ := newAuthSvc()
	long := make([]byte, maxPasswordLen+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.Register(context.Background(), RegisterInput{Email: "a@x.com", Nombre: "A", Password: string(long)})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("password > %d bytes -> ErrValidation (no 500 de bcrypt), obtuve %v", maxPasswordLen, err)
	}
}

func TestRegister_InvalidEmailIsValidationCode(t *testing.T) {
	svc, _ := newAuthSvc()
	_, err := svc.Register(context.Background(), RegisterInput{Email: "no-es-correo", Nombre: "A", Password: "S3guroPurpura!"})
	if !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("quiero ErrInvalidEmail, obtuve %v", err)
	}
	if ErrorCode(err) != "validation_failed" {
		t.Errorf("email invalido debe mapear a validation_failed, obtuve %q", ErrorCode(err))
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := newAuthSvc()
	in := RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"}
	if _, err := svc.Register(context.Background(), in); err != nil {
		t.Fatalf("primer registro fallo: %v", err)
	}
	_, err := svc.Register(context.Background(), in)
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("email duplicado -> ErrEmailTaken, obtuve %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newAuthSvc()
	reg := RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"}
	if _, err := svc.Register(context.Background(), reg); err != nil {
		t.Fatalf("registro fallo: %v", err)
	}
	res, err := svc.Login(context.Background(), LoginInput{Email: "Ana@Example.com", Password: "S3guroPurpura!"})
	if err != nil {
		t.Fatalf("login fallo: %v", err)
	}
	if res.Token.AccessToken != "token-id-1" {
		t.Errorf("token de login mal: %+v", res.Token)
	}
}

func TestLogin_WrongPasswordIsInvalidCredential(t *testing.T) {
	svc, _ := newAuthSvc()
	_, _ = svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	_, err := svc.Login(context.Background(), LoginInput{Email: "ana@example.com", Password: "incorrecta"})
	if !errors.Is(err, domain.ErrInvalidCredential) {
		t.Fatalf("password mala -> ErrInvalidCredential, obtuve %v", err)
	}
}

func TestLogin_UnknownUserDoesNotLeakExistence(t *testing.T) {
	svc, _ := newAuthSvc()
	_, err := svc.Login(context.Background(), LoginInput{Email: "nadie@example.com", Password: "x"})
	if !errors.Is(err, domain.ErrInvalidCredential) {
		t.Fatalf("usuario inexistente -> ErrInvalidCredential (no ErrUserNotFound), obtuve %v", err)
	}
}

func TestLogin_GoogleAccountHasNoLocalCredential(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Email: "g@gmail.com", Nombre: "G"}}
	if _, err := svc.AuthenticateWithGoogle(context.Background(), "idtok"); err != nil {
		t.Fatalf("alta google fallo: %v", err)
	}
	_, err := svc.Login(context.Background(), LoginInput{Email: "g@gmail.com", Password: "loquesea"})
	if !errors.Is(err, domain.ErrInvalidCredential) {
		t.Fatalf("cuenta google sin credencial -> ErrInvalidCredential, obtuve %v", err)
	}
}

func TestGoogle_NewUserCreated(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Email: "Carlos@Gmail.com", Nombre: "Carlos"}}
	res, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.User.ID != "id-1" || res.User.Email != "carlos@gmail.com" || res.User.AuthProvider != domain.AuthGoogle {
		t.Errorf("usuario google mal: %+v", res.User)
	}
}

func TestGoogle_ExistingGoogleReused(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Email: "carlos@gmail.com", Nombre: "Carlos"}}
	first, _ := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	second, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("segundo login google fallo: %v", err)
	}
	if first.User.ID != "id-1" || second.User.ID != "id-1" {
		t.Errorf("la cuenta google debe reutilizarse, no recrearse: %q / %q", first.User.ID, second.User.ID)
	}
}

func TestGoogle_EmailBelongsToPasswordAccount(t *testing.T) {
	svc, _ := newAuthSvc()
	_, _ = svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Email: "ana@example.com", Nombre: "Ana"}}
	_, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("email de cuenta password via google -> ErrEmailTaken, obtuve %v", err)
	}
}

func TestGoogle_InvalidIdTokenIsUnauthorized(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{err: errors.New("firma invalida")}
	_, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("idToken invalido -> ErrUnauthorized, obtuve %v", err)
	}
}
