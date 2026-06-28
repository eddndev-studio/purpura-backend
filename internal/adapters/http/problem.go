package httpadapter

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/eddndev-studio/purpura-backend/internal/app"
)

const (
	problemContentType = "application/problem+json; charset=utf-8"
	problemTypeBase    = "https://purpura.example/errors/"
)

// problemDoc es el cuerpo problem+json (RFC 7807; 04 seccion 4).
type problemDoc struct {
	Type     string              `json:"type"`
	Title    string              `json:"title"`
	Status   int                 `json:"status"`
	Detail   string              `json:"detail"`
	Instance string              `json:"instance"`
	Code     string              `json:"code"`
	Errors   []problemFieldError `json:"errors,omitempty"`
}

// problemFieldError es un error de validacion por campo (04 seccion 4: arreglo
// opcional `errors` en 400/422).
type problemFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// problemTitles son etiquetas estables por codigo. Se mantienen en ASCII
// (convencion de codigo del proyecto); el cliente mapea `code` a su propia
// etiqueta acentuada (04: el cliente no parsea prosa, usa `code`).
var problemTitles = map[string]string{
	"invalid_event_type":   "Tipo de evento invalido",
	"invalid_status":       "Estatus invalido",
	"invalid_reminder":     "Recordatorio invalido",
	"empty_description":    "Descripcion invalida",
	"invalid_location":     "Ubicacion invalida",
	"event_not_found":      "Evento no encontrado",
	"user_not_found":       "Usuario no encontrado",
	"email_taken":          "Correo ya registrado",
	"google_link_conflict": "Conflicto al vincular Google",
	"cannot_unlink_google": "No se puede desvincular Google",
	"email_not_verified":   "Correo de Google no verificado",
	"invalid_google_token": "Token de Google invalido",
	"invalid_credential":   "Credenciales invalidas",
	"validation_failed":    "Validacion fallida",
	"unauthorized":         "No autorizado",
	"bad_request":          "Solicitud mal formada",
	"method_not_allowed":   "Metodo no permitido",
	"payload_too_large":    "Carga demasiado grande",
	"internal_error":       "Error interno",
}

// statusForCode mapea el codigo estable al status HTTP (04 secciones 4.1/4.2).
func statusForCode(code string) int {
	switch code {
	case "invalid_event_type", "invalid_status", "invalid_reminder",
		"empty_description", "invalid_location", "validation_failed":
		return http.StatusUnprocessableEntity
	case "event_not_found", "user_not_found":
		return http.StatusNotFound
	case "email_taken", "google_link_conflict", "cannot_unlink_google":
		return http.StatusConflict
	case "invalid_credential", "unauthorized":
		return http.StatusUnauthorized
	case "email_not_verified":
		return http.StatusForbidden
	case "bad_request", "invalid_google_token":
		return http.StatusBadRequest
	case "method_not_allowed":
		return http.StatusMethodNotAllowed
	case "payload_too_large":
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

// writeError traduce un error de dominio/app a problem+json usando app.ErrorCode
// como fuente unica de codigos. El status se deriva del codigo. Para un 500 NO
// se filtra el mensaje interno (driver/SQL/libreria) al cliente: se registra
// server-side y se devuelve un detalle generico.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	code := app.ErrorCode(err)
	status := statusForCode(code)
	detail := err.Error()
	if status == http.StatusInternalServerError {
		slog.Error("error interno no controlado", "err", err.Error(), "path", r.URL.Path)
		detail = "error interno del servidor"
	}
	writeProblem(w, r, status, code, detail)
}

// writeProblem escribe el documento problem+json con el status y codigo dados.
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	writeProblemWithFields(w, r, status, code, detail, nil)
}

// writeProblemWithFields escribe problem+json incluyendo el arreglo `errors` por
// campo (p.ej. import atomico fallido; 04 seccion 5.12).
func writeProblemWithFields(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []problemFieldError) {
	title, ok := problemTitles[code]
	if !ok {
		title = problemTitles["internal_error"]
	}
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemDoc{
		Type:     problemTypeBase + code,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
		Errors:   fields,
	})
}
