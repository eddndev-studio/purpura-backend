// Package app contiene los casos de uso (servicios de aplicacion). Orquestan
// dominio + puertos: validan la entrada ya decodificada, invocan constructores
// y mutadores de dominio, sellan tiempo/id via puertos y traducen el resultado.
// Depende SOLO de internal/domain e internal/ports; nunca de un adaptador.
package app

import (
	"errors"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// Errores de la capa de aplicacion/transporte (no son invariantes de dominio).
// El adaptador HTTP los mapea a 422 validation_failed y 401 unauthorized
// (04 seccion 4.2). Se envuelven con fmt.Errorf("%w: detalle", ErrValidation)
// para conservar el sentinel y anadir contexto; los handlers usan errors.Is.
var (
	// ErrValidation indica entrada invalida en la capa de transporte: cuerpo
	// PATCH vacio, paginacion/orden invalidos, password corta, modo de import
	// atomic con items invalidos. Mapea a 422 validation_failed.
	ErrValidation = errors.New("validacion fallida")

	// ErrUnauthorized indica fallo de autorizacion ajeno a una credencial de
	// dominio: idToken de Google no verificable. Mapea a 401 unauthorized.
	ErrUnauthorized = errors.New("no autorizado")
)

// ErrorCode traduce un error (de dominio o de aplicacion) al codigo estable y
// machine-readable del contrato (04 seccion 4.1/4.2). Es la fuente unica de los
// codigos: ImportEvents lo usa para los errores por item, y el adaptador HTTP lo
// reutiliza para el campo `code` de problem+json (la asignacion de status HTTP
// vive en el adaptador). Un error no reconocido es internal_error.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidEventType):
		return "invalid_event_type"
	case errors.Is(err, domain.ErrInvalidStatus):
		return "invalid_status"
	case errors.Is(err, domain.ErrInvalidReminder):
		return "invalid_reminder"
	case errors.Is(err, domain.ErrEmptyDescription):
		return "empty_description"
	case errors.Is(err, domain.ErrInvalidLocation):
		return "invalid_location"
	case errors.Is(err, domain.ErrEventNotFound):
		return "event_not_found"
	case errors.Is(err, domain.ErrUserNotFound):
		return "user_not_found"
	case errors.Is(err, domain.ErrEmailTaken):
		return "email_taken"
	case errors.Is(err, domain.ErrGoogleLinkConflict):
		return "google_link_conflict"
	case errors.Is(err, domain.ErrCannotUnlinkGoogle):
		return "cannot_unlink_google"
	case errors.Is(err, domain.ErrEmailNotVerified):
		return "email_not_verified"
	case errors.Is(err, domain.ErrInvalidGoogleToken):
		return "invalid_google_token"
	case errors.Is(err, domain.ErrInvalidCredential):
		return "invalid_credential"
	// Las validaciones de NewUser (formato de email, nombre vacio, proveedor)
	// son entrada invalida del cliente: el contrato las trata como
	// validation_failed (04 seccion 5.2), no como codigos propios.
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrEmptyName),
		errors.Is(err, domain.ErrInvalidAuthProvider),
		errors.Is(err, ErrValidation):
		return "validation_failed"
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	default:
		return "internal_error"
	}
}
