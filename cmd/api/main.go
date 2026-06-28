// Command api es el composition root del backend: lee la configuracion, ensambla
// los adaptadores y casos de uso de adentro hacia afuera, y arranca el servidor
// HTTP con apagado ordenado. Es el unico paquete que conoce tipos concretos.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// tzdata embebe la base de zonas horarias en el binario, de modo que la
	// aritmetica de calendario de query_window funcione en imagenes minimas o
	// runners sin /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/eddndev-studio/purpura-backend/internal/adapters/auth"
	"github.com/eddndev-studio/purpura-backend/internal/adapters/email"
	httpadapter "github.com/eddndev-studio/purpura-backend/internal/adapters/http"
	"github.com/eddndev-studio/purpura-backend/internal/adapters/postgres"
	"github.com/eddndev-studio/purpura-backend/internal/adapters/sys"
	"github.com/eddndev-studio/purpura-backend/internal/app"
	"github.com/eddndev-studio/purpura-backend/internal/config"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

func main() {
	if err := run(); err != nil {
		slog.Error("el servidor termino con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Adaptadores driven (implementan los puertos).
	tokens, err := auth.NewJWTService(auth.JWTConfig{
		SigningMethod: cfg.JWTSigningMethod,
		Secret:        cfg.JWTSecret,
		PrivateKeyPEM: cfg.JWTPrivateKeyPEM,
		PublicKeyPEM:  cfg.JWTPublicKeyPEM,
		Issuer:        cfg.JWTIssuer,
		Audience:      cfg.JWTAudience,
		TTL:           cfg.JWTTTL,
	})
	if err != nil {
		return err
	}
	google := auth.NewGoogleVerifier(cfg.GoogleClientID)
	hasher := auth.NewBcryptHasher(cfg.BcryptCost)
	clock := sys.NewClock()
	ids := sys.NewUUIDGenerator()
	verifCodec := sys.NewVerificationTokenCodec()
	eventsRepo := postgres.NewEventRepository(pool)
	usersRepo := postgres.NewUserRepository(pool)
	verifTokens := postgres.NewVerificationTokenRepository(pool)

	// EmailSender: Resend si hay API key, si no un sender de log (no brickea el
	// despliegue; el enlace de verificacion queda en el log para local/staging).
	var emailSender ports.EmailSender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom)
	} else {
		logger.Warn("RESEND_API_KEY vacia: los correos de verificacion solo se registran en el log")
		emailSender = email.NewLogSender(logger)
	}

	// Casos de uso (dependen solo de puertos).
	eventSvc := &app.EventService{Events: eventsRepo, Clock: clock, IDs: ids}
	authSvc := &app.AuthService{
		Users:  usersRepo,
		Tokens: tokens,
		Google: google,
		Hasher: hasher,
		Clock:  clock,
		IDs:    ids,
	}
	verificationSvc := &app.VerificationService{
		Users:     usersRepo,
		Tokens:    verifTokens,
		Codec:     verifCodec,
		Email:     emailSender,
		Clock:     clock,
		IDs:       ids,
		TTL:       cfg.EmailVerificationTTL,
		VerifyURL: cfg.EmailVerifyURL,
	}

	// Adaptador driving (consume los casos de uso).
	router := httpadapter.NewRouter(httpadapter.Deps{
		Events:       eventSvc,
		Auth:         authSvc,
		Verification: verificationSvc,
		Tokens:       tokens,
		Pinger:       pool,
		CORSOrigins:  cfg.CORSOrigins,
		MaxBodyBytes: cfg.MaxBodyBytes,
		Logger:       logger,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return serve(ctx, srv, logger)
}

// serve arranca el servidor y lo apaga ordenadamente ante SIGINT/SIGTERM.
func serve(ctx context.Context, srv *http.Server, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("servidor escuchando", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("apagando servidor")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
