package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// --- fakes especificos de verificacion ---

type fakeVerifTokenRepo struct {
	byHash  map[string]*ports.VerificationToken
	created []*ports.VerificationToken
}

func newFakeVerifTokenRepo() *fakeVerifTokenRepo {
	return &fakeVerifTokenRepo{byHash: map[string]*ports.VerificationToken{}}
}

func (r *fakeVerifTokenRepo) Create(_ context.Context, t *ports.VerificationToken) error {
	cp := *t
	r.byHash[t.TokenHash] = &cp
	r.created = append(r.created, &cp)
	return nil
}

func (r *fakeVerifTokenRepo) FindByHash(_ context.Context, hash string) (*ports.VerificationToken, error) {
	t, ok := r.byHash[hash]
	if !ok {
		return nil, domain.ErrInvalidVerificationToken
	}
	cp := *t
	return &cp, nil
}

func (r *fakeVerifTokenRepo) MarkUsed(_ context.Context, id string, usedAt time.Time) (bool, error) {
	for _, t := range r.byHash {
		if t.ID == id {
			if t.UsedAt != nil {
				return false, nil // ya usado
			}
			u := usedAt
			t.UsedAt = &u
			return true, nil
		}
	}
	return false, nil
}

// fakeCodec genera un token fijo y hashea como "hash:"+crudo (deterministico).
type fakeCodec struct{ raw string }

func (c *fakeCodec) Mint() (string, string, error) { return c.raw, "hash:" + c.raw, nil }
func (c *fakeCodec) Hash(raw string) string        { return "hash:" + raw }

type fakeEmail struct {
	sent []ports.VerificationEmail
	err  error
}

func (e *fakeEmail) SendVerificationEmail(_ context.Context, msg ports.VerificationEmail) error {
	if e.err != nil {
		return e.err
	}
	e.sent = append(e.sent, msg)
	return nil
}

func newVerifService() (*VerificationService, *fakeUserRepo, *fakeVerifTokenRepo, *fakeEmail) {
	users := newFakeUserRepo()
	tokens := newFakeVerifTokenRepo()
	email := &fakeEmail{}
	svc := &VerificationService{
		Users:     users,
		Tokens:    tokens,
		Codec:     &fakeCodec{raw: "rawtok"},
		Email:     email,
		Clock:     fixedClock{t: fixedNow},
		IDs:       &seqIDGen{},
		TTL:       time.Hour,
		VerifyURL: "https://purpura.eddn.dev/verify",
	}
	return svc, users, tokens, email
}

func unverifiedUser() *domain.User {
	return &domain.User{ID: "u1", Email: "ana@example.com", Nombre: "Ana", AuthProvider: domain.AuthPassword}
}

func TestRequest_CreatesTokenAndSendsEmail(t *testing.T) {
	svc, users, tokens, email := newVerifService()
	users.put(unverifiedUser())

	if err := svc.RequestVerification(context.Background(), "u1"); err != nil {
		t.Fatalf("RequestVerification: %v", err)
	}
	if len(tokens.created) != 1 {
		t.Fatalf("tokens creados = %d, quiero 1", len(tokens.created))
	}
	tok := tokens.created[0]
	if tok.UserID != "u1" || tok.TokenHash != "hash:rawtok" {
		t.Errorf("token mal: userID=%q hash=%q", tok.UserID, tok.TokenHash)
	}
	if !tok.ExpiresAt.Equal(fixedNow.Add(time.Hour)) {
		t.Errorf("expiry = %v, quiero now+TTL", tok.ExpiresAt)
	}
	if len(email.sent) != 1 {
		t.Fatalf("correos enviados = %d, quiero 1", len(email.sent))
	}
	if email.sent[0].To != "ana@example.com" || !strings.Contains(email.sent[0].VerifyURL, "token=rawtok") {
		t.Errorf("correo mal: to=%q url=%q", email.sent[0].To, email.sent[0].VerifyURL)
	}
}

func TestRequest_AlreadyVerifiedIsNoOp(t *testing.T) {
	svc, users, tokens, email := newVerifService()
	u := unverifiedUser()
	u.EmailVerified = true
	users.put(u)

	if err := svc.RequestVerification(context.Background(), "u1"); err != nil {
		t.Fatalf("RequestVerification: %v", err)
	}
	if len(tokens.created) != 0 || len(email.sent) != 0 {
		t.Errorf("ya verificado: no debe crear token (%d) ni enviar correo (%d)", len(tokens.created), len(email.sent))
	}
}

func TestRequest_UnknownUser(t *testing.T) {
	svc, _, tokens, email := newVerifService()
	err := svc.RequestVerification(context.Background(), "nope")
	if err != domain.ErrUserNotFound {
		t.Fatalf("err = %v, quiero ErrUserNotFound", err)
	}
	if len(tokens.created) != 0 || len(email.sent) != 0 {
		t.Errorf("usuario inexistente no debe crear token ni enviar correo")
	}
}

// seedToken inserta un token ya hasheado para "rawtok" con la expiracion dada.
func seedToken(tokens *fakeVerifTokenRepo, id string, expiresAt time.Time, used *time.Time) {
	tokens.byHash["hash:rawtok"] = &ports.VerificationToken{
		ID: id, UserID: "u1", TokenHash: "hash:rawtok", ExpiresAt: expiresAt, UsedAt: used,
	}
}

func TestConfirm_HappyMarksVerifiedAndUsed(t *testing.T) {
	svc, users, tokens, _ := newVerifService()
	users.put(unverifiedUser())
	seedToken(tokens, "tok-1", fixedNow.Add(time.Hour), nil)

	if err := svc.ConfirmVerification(context.Background(), "rawtok"); err != nil {
		t.Fatalf("ConfirmVerification: %v", err)
	}
	u, _ := users.FindByID(context.Background(), "u1")
	if !u.EmailVerified {
		t.Errorf("el usuario debe quedar verificado")
	}
	if tokens.byHash["hash:rawtok"].UsedAt == nil {
		t.Errorf("el token debe quedar marcado como usado")
	}
}

func TestConfirm_EmptyToken(t *testing.T) {
	svc, _, _, _ := newVerifService()
	if err := svc.ConfirmVerification(context.Background(), ""); err != domain.ErrInvalidVerificationToken {
		t.Fatalf("err = %v, quiero ErrInvalidVerificationToken", err)
	}
}

func TestConfirm_UnknownToken(t *testing.T) {
	svc, _, _, _ := newVerifService()
	if err := svc.ConfirmVerification(context.Background(), "rawtok"); err != domain.ErrInvalidVerificationToken {
		t.Fatalf("err = %v, quiero ErrInvalidVerificationToken", err)
	}
}

func TestConfirm_AlreadyUsed(t *testing.T) {
	svc, users, tokens, _ := newVerifService()
	users.put(unverifiedUser())
	used := fixedNow.Add(-time.Minute)
	seedToken(tokens, "tok-1", fixedNow.Add(time.Hour), &used)

	if err := svc.ConfirmVerification(context.Background(), "rawtok"); err != domain.ErrInvalidVerificationToken {
		t.Fatalf("err = %v, quiero ErrInvalidVerificationToken (ya usado)", err)
	}
	u, _ := users.FindByID(context.Background(), "u1")
	if u.EmailVerified {
		t.Errorf("un token usado no debe verificar")
	}
}

func TestConfirm_Expired(t *testing.T) {
	svc, users, tokens, _ := newVerifService()
	users.put(unverifiedUser())
	seedToken(tokens, "tok-1", fixedNow.Add(-time.Minute), nil) // ya expirado

	if err := svc.ConfirmVerification(context.Background(), "rawtok"); err != domain.ErrVerificationTokenExpired {
		t.Fatalf("err = %v, quiero ErrVerificationTokenExpired", err)
	}
	u, _ := users.FindByID(context.Background(), "u1")
	if u.EmailVerified {
		t.Errorf("un token expirado no debe verificar")
	}
}
