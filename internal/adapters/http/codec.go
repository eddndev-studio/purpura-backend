package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/app"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// errMalformedJSON y errBodyTooLarge son sentinelas de transporte (no de
// dominio) que los handlers traducen a 400 bad_request y 413 payload_too_large.
var (
	errMalformedJSON = errors.New("bad_request")
	errBodyTooLarge  = errors.New("payload_too_large")
)

// ---------------------------------------------------------------------------
// Respuestas (forma plana, claves camelCase; 04 secciones 2.1/2.2).
// ---------------------------------------------------------------------------

type eventResponse struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	EventType     string    `json:"eventType"`
	ContactName   string    `json:"contactName"`
	ContactRef    string    `json:"contactRef"`
	LocationLat   float64   `json:"locationLat"`
	LocationLng   float64   `json:"locationLng"`
	LocationLabel string    `json:"locationLabel"`
	Description   string    `json:"description"`
	StartsAt      time.Time `json:"startsAt"`
	EventStatus   string    `json:"eventStatus"`
	ReminderType  string    `json:"reminderType"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func toEventResponse(e domain.Event) eventResponse {
	return eventResponse{
		ID:            e.ID,
		UserID:        e.UserID,
		EventType:     string(e.Type),
		ContactName:   e.Contact.Name,
		ContactRef:    e.Contact.Ref,
		LocationLat:   e.Location.Lat,
		LocationLng:   e.Location.Lng,
		LocationLabel: e.Location.Label,
		Description:   e.Description,
		StartsAt:      e.StartsAt,
		EventStatus:   string(e.Status),
		ReminderType:  string(e.Reminder),
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func toEventResponses(es []domain.Event) []eventResponse {
	out := make([]eventResponse, len(es))
	for i := range es {
		out[i] = toEventResponse(es[i])
	}
	return out
}

type userResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Nombre       string `json:"nombre"`
	AuthProvider string `json:"authProvider"`
	// GoogleLinked: la cuenta tiene Google adjunto (entra tambien por Google).
	// Derivado de google_sub != null; lo usa la app para mostrar Vincular/Desvincular.
	GoogleLinked bool      `json:"googleLinked"`
	CreatedAt    time.Time `json:"createdAt"`
}

type authResponse struct {
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresIn   int64        `json:"expiresIn"`
	User        userResponse `json:"user"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:           u.ID,
		Email:        u.Email,
		Nombre:       u.Nombre,
		AuthProvider: string(u.AuthProvider),
		GoogleLinked: u.GoogleLinked(),
		CreatedAt:    u.CreatedAt,
	}
}

func toAuthResponse(res app.AuthResult) authResponse {
	return authResponse{
		AccessToken: res.Token.AccessToken,
		TokenType:   res.Token.TokenType,
		ExpiresIn:   res.Token.ExpiresIn,
		User:        toUserResponse(res.User),
	}
}

type paginationResponse struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	TotalItems int    `json:"totalItems"`
	TotalPages int    `json:"totalPages"`
	Sort       string `json:"sort"`
}

type listResponse struct {
	Data       []eventResponse    `json:"data"`
	Pagination paginationResponse `json:"pagination"`
}

type exportResponse struct {
	SchemaVersion string          `json:"schemaVersion"`
	ExportedAt    time.Time       `json:"exportedAt"`
	UserID        string          `json:"userId"`
	Count         int             `json:"count"`
	Events        []eventResponse `json:"events"`
}

type importErrorResponse struct {
	Index  int    `json:"index"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type importSummaryResponse struct {
	Imported int                   `json:"imported"`
	Updated  int                   `json:"updated"`
	Skipped  int                   `json:"skipped"`
	Failed   int                   `json:"failed"`
	Errors   []importErrorResponse `json:"errors"`
}

func toImportSummaryResponse(s app.ImportSummary) importSummaryResponse {
	errs := make([]importErrorResponse, len(s.Errors))
	for i, e := range s.Errors {
		errs[i] = importErrorResponse{Index: e.Index, Code: e.Code, Detail: e.Detail}
	}
	return importSummaryResponse{
		Imported: s.Imported,
		Updated:  s.Updated,
		Skipped:  s.Skipped,
		Failed:   s.Failed,
		Errors:   errs,
	}
}

// ---------------------------------------------------------------------------
// Peticiones. El cast de cadena a tipo de dominio es directo; la VALIDACION del
// valor la hace el dominio (un enum invalido -> error de dominio -> 422).
// ---------------------------------------------------------------------------

type createEventRequest struct {
	EventType     string    `json:"eventType"`
	ContactName   string    `json:"contactName"`
	ContactRef    string    `json:"contactRef"`
	LocationLat   float64   `json:"locationLat"`
	LocationLng   float64   `json:"locationLng"`
	LocationLabel string    `json:"locationLabel"`
	Description   string    `json:"description"`
	StartsAt      time.Time `json:"startsAt"`
	ReminderType  string    `json:"reminderType"`
	// userId y eventStatus se ignoran deliberadamente si vienen en el cuerpo.
}

func (r createEventRequest) toInput() app.CreateEventInput {
	return app.CreateEventInput{
		Type:        domain.EventType(r.EventType),
		Contact:     domain.Contact{Name: r.ContactName, Ref: r.ContactRef},
		Location:    domain.Location{Lat: r.LocationLat, Lng: r.LocationLng, Label: r.LocationLabel},
		Description: r.Description,
		StartsAt:    r.StartsAt,
		Reminder:    domain.Reminder(r.ReminderType),
	}
}

// updateEventRequest usa punteros para distinguir "campo ausente" (nil) de
// "campo presente con valor vacio" (semantica PATCH; 04 seccion 5.8). Cada
// subcampo de contacto y ubicacion es independiente: enviar solo uno conserva
// los hermanos no enviados (la fusion ocurre en domain.Event.Edit).
type updateEventRequest struct {
	EventType     *string    `json:"eventType"`
	ContactName   *string    `json:"contactName"`
	ContactRef    *string    `json:"contactRef"`
	LocationLat   *float64   `json:"locationLat"`
	LocationLng   *float64   `json:"locationLng"`
	LocationLabel *string    `json:"locationLabel"`
	Description   *string    `json:"description"`
	StartsAt      *time.Time `json:"startsAt"`
	ReminderType  *string    `json:"reminderType"`
}

func (r updateEventRequest) toPatch() domain.EventPatch {
	p := domain.EventPatch{
		Description:   r.Description,
		StartsAt:      r.StartsAt,
		ContactName:   r.ContactName,
		ContactRef:    r.ContactRef,
		LocationLat:   r.LocationLat,
		LocationLng:   r.LocationLng,
		LocationLabel: r.LocationLabel,
	}
	if r.EventType != nil {
		t := domain.EventType(*r.EventType)
		p.Type = &t
	}
	if r.ReminderType != nil {
		rem := domain.Reminder(*r.ReminderType)
		p.Reminder = &rem
	}
	return p
}

type changeStatusRequest struct {
	EventStatus string `json:"eventStatus"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Nombre   string `json:"nombre"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type googleRequest struct {
	IDToken string `json:"idToken"`
}

// linkGoogleRequest es el cuerpo de POST /account/link-google: el idToken de
// Google que se adjunta a la cuenta autenticada.
type linkGoogleRequest struct {
	IDToken string `json:"idToken"`
}

type importEventRequest struct {
	ID            string    `json:"id"`
	EventType     string    `json:"eventType"`
	ContactName   string    `json:"contactName"`
	ContactRef    string    `json:"contactRef"`
	LocationLat   float64   `json:"locationLat"`
	LocationLng   float64   `json:"locationLng"`
	LocationLabel string    `json:"locationLabel"`
	Description   string    `json:"description"`
	StartsAt      time.Time `json:"startsAt"`
	EventStatus   *string   `json:"eventStatus"` // puntero: presencia => se respeta
	ReminderType  string    `json:"reminderType"`
}

type importRequest struct {
	Mode   string               `json:"mode"`
	Events []importEventRequest `json:"events"`
}

func (r importRequest) toInput() app.ImportInput {
	events := make([]app.ImportEventInput, len(r.Events))
	for i, e := range r.Events {
		var status *domain.EventStatus
		if e.EventStatus != nil {
			s := domain.EventStatus(*e.EventStatus)
			status = &s
		}
		events[i] = app.ImportEventInput{
			ID:          e.ID,
			Type:        domain.EventType(e.EventType),
			Contact:     domain.Contact{Name: e.ContactName, Ref: e.ContactRef},
			Location:    domain.Location{Lat: e.LocationLat, Lng: e.LocationLng, Label: e.LocationLabel},
			Description: e.Description,
			StartsAt:    e.StartsAt,
			Status:      status,
			Reminder:    domain.Reminder(e.ReminderType),
		}
	}
	return app.ImportInput{Mode: r.Mode, Events: events}
}

// ---------------------------------------------------------------------------
// Helpers de codec.
// ---------------------------------------------------------------------------

// decodeJSON decodifica el cuerpo en v acotando su tamano. Cuerpo demasiado
// grande -> errBodyTooLarge (413); JSON malformado o tipos invalidos -> errMalformedJSON (400).
func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errBodyTooLarge
		}
		return errMalformedJSON
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeDecodeError traduce los sentinelas de decodeJSON a problem+json.
func writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errBodyTooLarge):
		writeProblem(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "cuerpo demasiado grande")
	case errors.Is(err, errMalformedJSON):
		writeProblem(w, r, http.StatusBadRequest, "bad_request", "JSON malformado o tipos invalidos")
	default:
		writeError(w, r, err)
	}
}
