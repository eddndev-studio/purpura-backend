package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// JWTConfig parametriza el TokenService (06/07 seccion 6.2). Para HS256 basta
// Secret; para RS256 se requieren las llaves PEM.
type JWTConfig struct {
	SigningMethod string // "HS256" (default) | "RS256"
	Secret        string // HS256
	PrivateKeyPEM string // RS256 (firma)
	PublicKeyPEM  string // RS256 (verificacion)
	Issuer        string
	Audience      string
	TTL           time.Duration
}

// JWTService implementa ports.TokenService emitiendo y verificando el JWT propio
// de Purpura con el claim set de 04 seccion 3.2.
type JWTService struct {
	method    jwt.SigningMethod
	signKey   any
	verifyKey any
	issuer    string
	audience  string
	ttl       time.Duration

	// inyectables para pruebas deterministas.
	now   func() time.Time
	newID func() string
}

var _ ports.TokenService = (*JWTService)(nil)

// NewJWTService construye el servicio desde la configuracion. Valida que los
// secretos/llaves del metodo elegido esten presentes y bien formados.
func NewJWTService(cfg JWTConfig) (*JWTService, error) {
	s := &JWTService{
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.TTL,
		now:      func() time.Time { return time.Now().UTC() },
		newID:    uuid.NewString,
	}
	if s.ttl <= 0 {
		return nil, errors.New("jwt: TTL debe ser positivo")
	}

	switch cfg.SigningMethod {
	case "", "HS256":
		if cfg.Secret == "" {
			return nil, errors.New("jwt: HS256 requiere Secret")
		}
		s.method = jwt.SigningMethodHS256
		s.signKey = []byte(cfg.Secret)
		s.verifyKey = []byte(cfg.Secret)
	case "RS256":
		priv, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("jwt: llave privada RS256 invalida: %w", err)
		}
		pub, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("jwt: llave publica RS256 invalida: %w", err)
		}
		s.method = jwt.SigningMethodRS256
		s.signKey = priv
		s.verifyKey = pub
	default:
		return nil, fmt.Errorf("jwt: metodo de firma no soportado: %q", cfg.SigningMethod)
	}
	return s, nil
}

// Issue emite un access token para el usuario.
func (s *JWTService) Issue(_ context.Context, u *domain.User) (ports.IssuedToken, error) {
	now := s.now()
	exp := now.Add(s.ttl)
	claims := jwt.MapClaims{
		"sub":          u.ID,
		"email":        u.Email,
		"authProvider": string(u.AuthProvider),
		"iss":          s.issuer,
		"aud":          s.audience,
		"iat":          now.Unix(),
		"exp":          exp.Unix(),
		"jti":          s.newID(),
	}
	signed, err := jwt.NewWithClaims(s.method, claims).SignedString(s.signKey)
	if err != nil {
		return ports.IssuedToken{}, err
	}
	return ports.IssuedToken{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.ttl.Seconds()),
	}, nil
}

// Verify valida firma, metodo, iss, aud y expiracion. No consulta la BD. Token
// invalido o expirado -> error (el middleware lo traduce a 401).
func (s *JWTService) Verify(_ context.Context, accessToken string) (ports.Claims, error) {
	parsed, err := jwt.Parse(accessToken, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != s.method.Alg() {
			return nil, fmt.Errorf("jwt: alg inesperado: %s", t.Method.Alg())
		}
		return s.verifyKey, nil
	},
		jwt.WithValidMethods([]string{s.method.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return ports.Claims{}, err
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return ports.Claims{}, errors.New("jwt: claims invalidos")
	}
	return ports.Claims{
		Subject:      claimString(mc, "sub"),
		Email:        claimString(mc, "email"),
		AuthProvider: claimString(mc, "authProvider"),
		JTI:          claimString(mc, "jti"),
	}, nil
}

func claimString(mc jwt.MapClaims, key string) string {
	if v, ok := mc[key].(string); ok {
		return v
	}
	return ""
}
