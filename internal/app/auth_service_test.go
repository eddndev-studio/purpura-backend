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
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-g", Email: "g@gmail.com", Nombre: "G"}}
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
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-carlos", Email: "Carlos@Gmail.com", Nombre: "Carlos"}}
	res, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.User.ID != "id-1" || res.User.Email != "carlos@gmail.com" || res.User.AuthProvider != domain.AuthGoogle {
		t.Errorf("usuario google mal: %+v", res.User)
	}
	// La cuenta google nace sellada con su sub (llave de vinculacion).
	if res.User.GoogleSub == nil || *res.User.GoogleSub != "sub-carlos" {
		t.Errorf("la cuenta google debe nacer con google_sub=sub-carlos: %+v", res.User.GoogleSub)
	}
}

func TestGoogle_ExistingGoogleReusedBySub(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-carlos", Email: "carlos@gmail.com", Nombre: "Carlos"}}
	first, _ := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	second, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("segundo login google fallo: %v", err)
	}
	if first.User.ID != "id-1" || second.User.ID != "id-1" {
		t.Errorf("la cuenta google debe reutilizarse, no recrearse: %q / %q", first.User.ID, second.User.ID)
	}
}

// Aunque el email de Google cambie, el login sigue resolviendo a la misma cuenta
// por el sub inmutable (no se crea una cuenta nueva ni se rechaza).
func TestGoogle_ReusedBySubEvenIfEmailChanged(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-x", Email: "viejo@gmail.com", Nombre: "X"}}
	first, _ := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-x", Email: "nuevo@gmail.com", Nombre: "X"}}
	second, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("login por sub con email nuevo fallo: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Errorf("mismo sub debe ser la misma cuenta pese al email distinto: %q / %q", first.User.ID, second.User.ID)
	}
}

func TestGoogle_EmailBelongsToPasswordAccount(t *testing.T) {
	svc, _ := newAuthSvc()
	_, _ = svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-ana", Email: "ana@example.com", Nombre: "Ana"}}
	_, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("email de cuenta password via google (sin vinculo) -> ErrEmailTaken, obtuve %v", err)
	}
}

// Cuenta Google legacy (creada por email antes del llaveo por sub, google_sub
// nil): el primer login con sub la retro-rellena y entra a la MISMA cuenta.
func TestGoogle_LegacyAccountRetrofillsSub(t *testing.T) {
	svc, repo := newAuthSvc()
	legacy := &domain.User{ID: "legacy-1", Email: "legacy@gmail.com", Nombre: "Legacy", AuthProvider: domain.AuthGoogle}
	repo.put(legacy)
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-legacy", Email: "legacy@gmail.com", Nombre: "Legacy"}}
	res, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil {
		t.Fatalf("retro-fill fallo: %v", err)
	}
	if res.User.ID != "legacy-1" {
		t.Errorf("debe entrar a la cuenta legacy, no crear otra: %q", res.User.ID)
	}
	stored, _ := repo.FindByGoogleSub(context.Background(), "sub-legacy")
	if stored == nil || stored.ID != "legacy-1" {
		t.Errorf("el sub debe quedar retro-rellenado en la cuenta legacy")
	}
}

func TestLinkGoogle_AttachesToPasswordAccount(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-ana", Email: "ana@example.com", Nombre: "Ana"}}
	u, err := svc.LinkGoogle(context.Background(), reg.User.ID, "idtok")
	if err != nil {
		t.Fatalf("LinkGoogle fallo: %v", err)
	}
	if !u.GoogleLinked() || *u.GoogleSub != "sub-ana" {
		t.Errorf("la cuenta debe quedar vinculada a sub-ana: %+v", u.GoogleSub)
	}
	// Ahora puede entrar por Google (resuelve por sub a la MISMA cuenta password).
	login, err := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	if err != nil || login.User.ID != reg.User.ID {
		t.Errorf("tras vincular, el login Google debe entrar a la cuenta password: id=%q err=%v", login.User.ID, err)
	}
}

func TestLinkGoogle_IdempotentSameSub(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-ana", Email: "ana@example.com", Nombre: "Ana"}}
	if _, err := svc.LinkGoogle(context.Background(), reg.User.ID, "idtok"); err != nil {
		t.Fatalf("primer link fallo: %v", err)
	}
	if _, err := svc.LinkGoogle(context.Background(), reg.User.ID, "idtok"); err != nil {
		t.Errorf("re-vincular el MISMO sub debe ser idempotente, obtuve %v", err)
	}
}

func TestLinkGoogle_AccountAlreadyHasDifferentGoogle(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-1", Email: "ana@example.com", Nombre: "Ana"}}
	_, _ = svc.LinkGoogle(context.Background(), reg.User.ID, "idtok")
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-2", Email: "ana@example.com", Nombre: "Ana"}}
	_, err := svc.LinkGoogle(context.Background(), reg.User.ID, "idtok")
	if !errors.Is(err, domain.ErrGoogleLinkConflict) {
		t.Fatalf("vincular un segundo Google distinto -> ErrGoogleLinkConflict, obtuve %v", err)
	}
}

func TestLinkGoogle_SubTakenByAnotherAccount(t *testing.T) {
	svc, _ := newAuthSvc()
	// Cuenta A nace de Google con sub-shared.
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-shared", Email: "a@gmail.com", Nombre: "A"}}
	_, _ = svc.AuthenticateWithGoogle(context.Background(), "idtok")
	// Cuenta B (password) intenta vincular el MISMO sub.
	regB, _ := svc.Register(context.Background(), RegisterInput{Email: "b@example.com", Nombre: "B", Password: "S3guroPurpura!"})
	_, err := svc.LinkGoogle(context.Background(), regB.User.ID, "idtok")
	if !errors.Is(err, domain.ErrGoogleLinkConflict) {
		t.Fatalf("vincular un sub ya usado por otra cuenta -> ErrGoogleLinkConflict, obtuve %v", err)
	}
}

func TestLinkGoogle_InvalidIdTokenIsUnauthorized(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{err: errors.New("firma invalida")}
	_, err := svc.LinkGoogle(context.Background(), reg.User.ID, "idtok")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("idToken invalido al vincular -> ErrUnauthorized, obtuve %v", err)
	}
}

func TestUnlinkGoogle_WithPasswordSucceeds(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-ana", Email: "ana@example.com", Nombre: "Ana"}}
	_, _ = svc.LinkGoogle(context.Background(), reg.User.ID, "idtok")
	u, err := svc.UnlinkGoogle(context.Background(), reg.User.ID)
	if err != nil {
		t.Fatalf("UnlinkGoogle fallo: %v", err)
	}
	if u.GoogleLinked() {
		t.Errorf("la cuenta debe quedar desvinculada: %+v", u.GoogleSub)
	}
}

// Una cuenta de ORIGEN Google no tiene contrasena: desvincular la dejaria sin
// acceso -> se rechaza.
func TestUnlinkGoogle_NoPasswordRejected(t *testing.T) {
	svc, _ := newAuthSvc()
	svc.Google = fakeGoogleVerifier{identity: ports.GoogleIdentity{Sub: "sub-g", Email: "g@gmail.com", Nombre: "G"}}
	res, _ := svc.AuthenticateWithGoogle(context.Background(), "idtok")
	_, err := svc.UnlinkGoogle(context.Background(), res.User.ID)
	if !errors.Is(err, domain.ErrCannotUnlinkGoogle) {
		t.Fatalf("desvincular sin contrasena -> ErrCannotUnlinkGoogle, obtuve %v", err)
	}
}

func TestUnlinkGoogle_IdempotentWhenNotLinked(t *testing.T) {
	svc, _ := newAuthSvc()
	reg, _ := svc.Register(context.Background(), RegisterInput{Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!"})
	u, err := svc.UnlinkGoogle(context.Background(), reg.User.ID)
	if err != nil {
		t.Errorf("desvincular una cuenta sin Google debe ser no-op, obtuve %v", err)
	}
	if u.GoogleLinked() {
		t.Errorf("no deberia estar vinculada")
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

func TestDeleteAccount_RemovesUserAndCredential(t *testing.T) {
	svc, repo := newAuthSvc()
	res, err := svc.Register(context.Background(), RegisterInput{
		Email: "ana@example.com", Nombre: "Ana", Password: "S3guroPurpura!",
	})
	if err != nil {
		t.Fatalf("registro previo fallo: %v", err)
	}
	id := res.User.ID

	if err := svc.DeleteAccount(context.Background(), id); err != nil {
		t.Fatalf("DeleteAccount error inesperado: %v", err)
	}
	// La cuenta desaparece...
	if _, err := repo.FindByID(context.Background(), id); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("usuario deberia estar borrado, err=%v", err)
	}
	// ...y su credencial cae con ella (cascada).
	if _, err := repo.GetPasswordHash(context.Background(), id); !errors.Is(err, domain.ErrInvalidCredential) {
		t.Errorf("credencial deberia caer en cascada, err=%v", err)
	}
}

func TestDeleteAccount_UnknownUser_NotFound(t *testing.T) {
	svc, _ := newAuthSvc()
	if err := svc.DeleteAccount(context.Background(), "fantasma"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("cuenta inexistente -> ErrUserNotFound, obtuve %v", err)
	}
}
