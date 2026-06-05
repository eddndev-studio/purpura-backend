package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

func hsConfig() JWTConfig {
	return JWTConfig{
		SigningMethod: "HS256",
		Secret:        "secreto-de-prueba-0123456789",
		Issuer:        "purpura-backend",
		Audience:      "purpura-app",
		TTL:           time.Hour,
	}
}

func testUser() *domain.User {
	return &domain.User{ID: "u1", Email: "ana@example.com", AuthProvider: domain.AuthPassword}
}

func TestJWTService_HS256RoundTrip(t *testing.T) {
	s, err := NewJWTService(hsConfig())
	if err != nil {
		t.Fatalf("NewJWTService fallo: %v", err)
	}
	tok, err := s.Issue(context.Background(), testUser())
	if err != nil {
		t.Fatalf("Issue fallo: %v", err)
	}
	if tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 {
		t.Errorf("token mal formado: %+v", tok)
	}
	claims, err := s.Verify(context.Background(), tok.AccessToken)
	if err != nil {
		t.Fatalf("Verify fallo: %v", err)
	}
	if claims.Subject != "u1" || claims.Email != "ana@example.com" || claims.AuthProvider != "password" {
		t.Errorf("claims mal: %+v", claims)
	}
	if claims.JTI == "" {
		t.Errorf("jti deberia estar presente")
	}
}

func TestJWTService_RejectsExpired(t *testing.T) {
	s, _ := NewJWTService(hsConfig())
	s.now = func() time.Time { return time.Now().Add(-2 * time.Hour) } // emite ya expirado
	tok, _ := s.Issue(context.Background(), testUser())
	if _, err := s.Verify(context.Background(), tok.AccessToken); err == nil {
		t.Fatalf("un token expirado debe rechazarse")
	}
}

func TestJWTService_RejectsWrongAudienceAndSignature(t *testing.T) {
	issuer, _ := NewJWTService(hsConfig())
	tok, _ := issuer.Issue(context.Background(), testUser())

	wrongAud := hsConfig()
	wrongAud.Audience = "otra-app"
	vAud, _ := NewJWTService(wrongAud)
	if _, err := vAud.Verify(context.Background(), tok.AccessToken); err == nil {
		t.Errorf("audience distinta debe rechazarse")
	}

	wrongKey := hsConfig()
	wrongKey.Secret = "otro-secreto-totalmente-distinto"
	vKey, _ := NewJWTService(wrongKey)
	if _, err := vKey.Verify(context.Background(), tok.AccessToken); err == nil {
		t.Errorf("firma con otro secreto debe rechazarse")
	}
}

func TestJWTService_RS256RoundTrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("no se pudo generar llave RSA: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	s, err := NewJWTService(JWTConfig{
		SigningMethod: "RS256",
		PrivateKeyPEM: string(privPEM),
		PublicKeyPEM:  string(pubPEM),
		Issuer:        "purpura-backend",
		Audience:      "purpura-app",
		TTL:           time.Hour,
	})
	if err != nil {
		t.Fatalf("NewJWTService RS256 fallo: %v", err)
	}
	tok, err := s.Issue(context.Background(), testUser())
	if err != nil {
		t.Fatalf("Issue RS256 fallo: %v", err)
	}
	claims, err := s.Verify(context.Background(), tok.AccessToken)
	if err != nil || claims.Subject != "u1" {
		t.Fatalf("Verify RS256 fallo: %v claims=%+v", err, claims)
	}
}

func TestNewJWTService_ConfigErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  JWTConfig
	}{
		{"HS256 sin secreto", JWTConfig{SigningMethod: "HS256", TTL: time.Hour}},
		{"metodo no soportado", JWTConfig{SigningMethod: "none", Secret: "x", TTL: time.Hour}},
		{"TTL no positivo", JWTConfig{SigningMethod: "HS256", Secret: "x"}},
		{"RS256 PEM invalido", JWTConfig{SigningMethod: "RS256", PrivateKeyPEM: "no-pem", PublicKeyPEM: "no-pem", TTL: time.Hour}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewJWTService(tc.cfg); err == nil {
				t.Errorf("se esperaba error de configuracion")
			}
		})
	}
}
