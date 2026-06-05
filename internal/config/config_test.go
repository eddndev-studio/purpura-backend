package config

import (
	"testing"
	"time"
)

// setEnv fija variables y las limpia al terminar (t.Setenv las restaura).
func baseEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("GOOGLE_CLIENT_ID", "client.apps.googleusercontent.com")
	t.Setenv("JWT_SECRET", "un-secreto-de-prueba")
}

func TestLoad_DefaultsAndRequired(t *testing.T) {
	baseEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load fallo: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, quiero :8080", cfg.Addr)
	}
	if cfg.JWTSigningMethod != "HS256" || cfg.JWTIssuer != "purpura-backend" || cfg.JWTAudience != "purpura-app" {
		t.Errorf("defaults JWT mal: %+v", cfg)
	}
	if cfg.JWTTTL != 86400*time.Second {
		t.Errorf("TTL = %s", cfg.JWTTTL)
	}
	if cfg.BcryptCost != 12 || cfg.MaxBodyBytes != 1048576 {
		t.Errorf("defaults numericos mal: cost=%d max=%d", cfg.BcryptCost, cfg.MaxBodyBytes)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("CORS default = %v", cfg.CORSOrigins)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	cases := []struct {
		name string
		drop string
	}{
		{"sin DATABASE_URL", "DATABASE_URL"},
		{"sin GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_ID"},
		{"sin JWT_SECRET en HS256", "JWT_SECRET"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv(tc.drop, "")
			if _, err := Load(); err == nil {
				t.Errorf("se esperaba error por %s ausente", tc.drop)
			}
		})
	}
}

func TestLoad_RS256RequiresKeys(t *testing.T) {
	baseEnv(t)
	t.Setenv("JWT_SIGNING_METHOD", "RS256")
	if _, err := Load(); err == nil {
		t.Errorf("RS256 sin llaves debe fallar")
	}
}

func TestLoad_MalformedNumber(t *testing.T) {
	baseEnv(t)
	t.Setenv("BCRYPT_COST", "no-numero")
	if _, err := Load(); err == nil {
		t.Errorf("BCRYPT_COST no numerico debe fallar")
	}
}

func TestLoad_CustomCORS(t *testing.T) {
	baseEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.com, https://b.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[1] != "https://b.com" {
		t.Errorf("CORS = %v", cfg.CORSOrigins)
	}
}
