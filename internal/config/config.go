// Package config carga la configuracion del backend desde variables de entorno
// (07 seccion 6.2). No tiene valores por defecto inseguros: los secretos del
// metodo de firma elegido y la URL de la BD son obligatorios. Expone campos
// primitivos; el composition root (cmd/api) arma los structs de los adaptadores.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es la configuracion resuelta del proceso.
type Config struct {
	Addr        string
	DatabaseURL string

	JWTSigningMethod string
	JWTSecret        string
	JWTPrivateKeyPEM string
	JWTPublicKeyPEM  string
	JWTIssuer        string
	JWTAudience      string
	JWTTTL           time.Duration

	GoogleClientID string
	BcryptCost     int

	// Verificacion de correo (Fase 2). Sin secretos obligatorios: si ResendAPIKey
	// esta vacia, el composition root usa un EmailSender de log (no brickea el
	// despliegue). EmailFrom debe ser un remitente verificado en Resend.
	ResendAPIKey         string
	EmailFrom            string
	EmailVerifyURL       string
	EmailVerificationTTL time.Duration

	CORSOrigins  []string
	LogLevel     slog.Level
	MaxBodyBytes int64
}

// Load lee y valida la configuracion. Devuelve error (no panic) ante una
// variable obligatoria ausente o un valor malformado.
func Load() (Config, error) {
	cfg := Config{
		Addr:             ":" + getenv("PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSigningMethod: getenv("JWT_SIGNING_METHOD", "HS256"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		JWTPrivateKeyPEM: os.Getenv("JWT_PRIVATE_KEY"),
		JWTPublicKeyPEM:  os.Getenv("JWT_PUBLIC_KEY"),
		JWTIssuer:        getenv("JWT_ISSUER", "purpura-backend"),
		JWTAudience:      getenv("JWT_AUDIENCE", "purpura-app"),
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		EmailFrom:        getenv("EMAIL_FROM", "noreply@purpura.eddn.dev"),
		EmailVerifyURL:   getenv("EMAIL_VERIFY_URL", "https://purpura.eddn.dev/verify"),
		CORSOrigins:      splitCSV(getenv("CORS_ALLOWED_ORIGINS", "*")),
		LogLevel:         parseLevel(getenv("LOG_LEVEL", "info")),
	}

	verifTTL, err := getenvInt("EMAIL_VERIFICATION_TTL_SECONDS", 86400)
	if err != nil {
		return Config{}, err
	}
	cfg.EmailVerificationTTL = time.Duration(verifTTL) * time.Second

	ttl, err := getenvInt("JWT_TTL_SECONDS", 86400)
	if err != nil {
		return Config{}, err
	}
	cfg.JWTTTL = time.Duration(ttl) * time.Second

	cost, err := getenvInt("BCRYPT_COST", 12)
	if err != nil {
		return Config{}, err
	}
	cfg.BcryptCost = cost

	maxBody, err := getenvInt64("MAX_BODY_BYTES", 1048576)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxBodyBytes = maxBody

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL es obligatoria")
	}
	if c.GoogleClientID == "" {
		return fmt.Errorf("config: GOOGLE_CLIENT_ID es obligatoria")
	}
	switch c.JWTSigningMethod {
	case "HS256":
		if c.JWTSecret == "" {
			return fmt.Errorf("config: JWT_SECRET es obligatoria con HS256")
		}
	case "RS256":
		if c.JWTPrivateKeyPEM == "" || c.JWTPublicKeyPEM == "" {
			return fmt.Errorf("config: JWT_PRIVATE_KEY y JWT_PUBLIC_KEY son obligatorias con RS256")
		}
	default:
		return fmt.Errorf("config: JWT_SIGNING_METHOD invalido: %q", c.JWTSigningMethod)
	}
	if c.JWTTTL <= 0 {
		return fmt.Errorf("config: JWT_TTL_SECONDS debe ser positivo")
	}
	// Un TTL <= 0 mintaria tokens ya expirados (toda confirmacion daria 410) sin un
	// error de arranque que lo delate: se valida igual que JWTTTL.
	if c.EmailVerificationTTL <= 0 {
		return fmt.Errorf("config: EMAIL_VERIFICATION_TTL_SECONDS debe ser positivo")
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalido: %q", key, v)
	}
	return n, nil
}

func getenvInt64(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalido: %q", key, v)
	}
	return n, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
