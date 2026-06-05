package httpadapter

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// recoverer captura panics, los registra y responde 500 en problem+json, de modo
// que un panic en un handler no tumbe el proceso.
func (d Deps) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				d.Logger.Error("panic en handler",
					"panic", rec,
					"path", r.URL.Path,
					"request_id", middleware.GetReqID(r.Context()),
				)
				writeProblem(w, r, http.StatusInternalServerError, "internal_error", "error interno del servidor")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captura el status para el logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logging emite un log estructurado por peticion. No registra cuerpos ni
// secretos (no loggea Authorization, password ni idToken).
func (d Deps) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		d.Logger.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
			"sub", userIDFrom(r),
		)
	})
}

// cors responde el preflight y fija las cabeceras CORS segun CORSOrigins. La app
// Android no requiere CORS; se incluye para herramientas web/diagnostico.
func (d Deps) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed := d.allowedOrigin(origin); allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedOrigin devuelve el origen a reflejar: "*" si la lista esta vacia o
// contiene "*"; el origen si esta en la lista; "" si no se permite.
func (d Deps) allowedOrigin(origin string) string {
	if len(d.CORSOrigins) == 0 {
		return "*"
	}
	for _, o := range d.CORSOrigins {
		if o == "*" {
			return "*"
		}
		if o == origin {
			return origin
		}
	}
	return ""
}

// authMiddleware exige un JWT Bearer valido y, si lo es, inyecta los claims
// (sobre todo sub) en el contexto. Ausente/invalido/expirado -> 401.
func (d Deps) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeProblem(w, r, http.StatusUnauthorized, "unauthorized", "falta el token Bearer")
			return
		}
		claims, err := d.Tokens.Verify(r.Context(), token)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "unauthorized", "token invalido o expirado")
			return
		}
		next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return "", false
	}
	return token, true
}
