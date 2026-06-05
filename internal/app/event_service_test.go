package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

var fixedNow = utc(2026, time.June, 5, 12)

func newEventSvc() (*EventService, *fakeEventRepo, *seqIDGen) {
	repo := newFakeEventRepo()
	ids := &seqIDGen{}
	return &EventService{Events: repo, Clock: fixedClock{t: fixedNow}, IDs: ids}, repo, ids
}

func validCreate() CreateEventInput {
	return CreateEventInput{
		Type:        domain.TypeJunta,
		Contact:     domain.Contact{Name: "Maria", Ref: "ref-1"},
		Location:    domain.Location{Lat: 19.43, Lng: -99.13, Label: "CDMX"},
		Description: "Revision de avance",
		StartsAt:    utc(2026, time.June, 10, 15),
		Reminder:    domain.ReminderTenMinutes,
	}
}

func TestCreateEvent_SealsAndForcesPendiente(t *testing.T) {
	svc, _, _ := newEventSvc()
	e, err := svc.CreateEvent(context.Background(), "user-1", validCreate())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if e.ID != "id-1" {
		t.Errorf("ID = %q, quiero id-1", e.ID)
	}
	if e.UserID != "user-1" {
		t.Errorf("UserID = %q, quiero user-1 (del claim sub)", e.UserID)
	}
	if e.Status != domain.StatusPendiente {
		t.Errorf("Status = %q, quiero pendiente (siempre inicial)", e.Status)
	}
	if !e.CreatedAt.Equal(fixedNow) || !e.UpdatedAt.Equal(fixedNow) {
		t.Errorf("created/updated = %s/%s, quiero %s", e.CreatedAt, e.UpdatedAt, fixedNow)
	}
}

func TestCreateEvent_InvalidPropagatesDomainError(t *testing.T) {
	svc, _, _ := newEventSvc()
	in := validCreate()
	in.Description = "   "
	_, err := svc.CreateEvent(context.Background(), "user-1", in)
	if !errors.Is(err, domain.ErrEmptyDescription) {
		t.Fatalf("quiero ErrEmptyDescription, obtuve %v", err)
	}
}

func TestGetEvent_ForeignIsNotFound(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())

	if _, err := svc.GetEvent(context.Background(), "user-1", created.ID); err != nil {
		t.Fatalf("propio deberia encontrarse: %v", err)
	}
	if _, err := svc.GetEvent(context.Background(), "user-2", created.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Fatalf("ajeno -> ErrEventNotFound, obtuve %v", err)
	}
}

func TestUpdateEvent_PartialPreservesAndReseals(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())

	// Avanza el reloj para verificar el re-sello de UpdatedAt.
	later := utc(2026, time.June, 6, 9)
	svc.Clock = fixedClock{t: later}

	in := UpdateEventInput{Patch: domain.EventPatch{Description: ptr("Nueva descripcion")}}
	updated, err := svc.UpdateEvent(context.Background(), "user-1", created.ID, in)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if updated.Description != "Nueva descripcion" {
		t.Errorf("descripcion no aplicada: %q", updated.Description)
	}
	if updated.Type != created.Type || updated.StartsAt != created.StartsAt {
		t.Errorf("campos ausentes no deben cambiar")
	}
	if !updated.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %s, quiero re-sellado %s", updated.UpdatedAt, later)
	}
	if !updated.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt no debe cambiar en update")
	}
}

func TestUpdateEvent_EmptyPatchIsValidationError(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())
	_, err := svc.UpdateEvent(context.Background(), "user-1", created.ID, UpdateEventInput{})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("cuerpo vacio -> ErrValidation, obtuve %v", err)
	}
}

func TestUpdateEvent_RevalidatesLocation(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())
	bad := UpdateEventInput{Patch: domain.EventPatch{LocationLat: ptr(200.0), LocationLng: ptr(0.0)}}
	_, err := svc.UpdateEvent(context.Background(), "user-1", created.ID, bad)
	if !errors.Is(err, domain.ErrInvalidLocation) {
		t.Fatalf("quiero ErrInvalidLocation, obtuve %v", err)
	}
}

func TestUpdateEvent_ForeignIsNotFound(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())
	in := UpdateEventInput{Patch: domain.EventPatch{Description: ptr("x")}}
	_, err := svc.UpdateEvent(context.Background(), "user-2", created.ID, in)
	if !errors.Is(err, domain.ErrEventNotFound) {
		t.Fatalf("ajeno -> ErrEventNotFound, obtuve %v", err)
	}
}

func TestChangeStatus_ResealsAndValidates(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())
	later := utc(2026, time.June, 7, 8)
	svc.Clock = fixedClock{t: later}

	got, err := svc.ChangeStatus(context.Background(), "user-1", created.ID, domain.StatusRealizado)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Status != domain.StatusRealizado || !got.UpdatedAt.Equal(later) {
		t.Errorf("estatus/updated no aplicados: %q / %s", got.Status, got.UpdatedAt)
	}
	if _, err := svc.ChangeStatus(context.Background(), "user-1", created.ID, domain.EventStatus("zzz")); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("estatus invalido -> ErrInvalidStatus, obtuve %v", err)
	}
}

func TestDeleteEvent_OwnAndForeign(t *testing.T) {
	svc, _, _ := newEventSvc()
	created, _ := svc.CreateEvent(context.Background(), "user-1", validCreate())
	if err := svc.DeleteEvent(context.Background(), "user-2", created.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Fatalf("ajeno -> ErrEventNotFound, obtuve %v", err)
	}
	if err := svc.DeleteEvent(context.Background(), "user-1", created.ID); err != nil {
		t.Fatalf("propio deberia borrarse: %v", err)
	}
	if err := svc.DeleteEvent(context.Background(), "user-1", created.ID); !errors.Is(err, domain.ErrEventNotFound) {
		t.Fatalf("ya borrado -> ErrEventNotFound, obtuve %v", err)
	}
}

// seedAt crea un evento del usuario con un startsAt y tipo dados.
func seedAt(t *testing.T, svc *EventService, userID string, startsAt time.Time, typ domain.EventType) *domain.Event {
	t.Helper()
	in := validCreate()
	in.StartsAt = startsAt
	in.Type = typ
	e, err := svc.CreateEvent(context.Background(), userID, in)
	if err != nil {
		t.Fatalf("seed fallo: %v", err)
	}
	return e
}

func TestQueryEvents_WindowPaginationSort(t *testing.T) {
	svc, _, _ := newEventSvc()
	seedAt(t, svc, "user-1", utc(2026, time.June, 3, 10), domain.TypeJunta)
	seedAt(t, svc, "user-1", utc(2026, time.June, 10, 10), domain.TypeCita)
	seedAt(t, svc, "user-1", utc(2026, time.June, 20, 10), domain.TypeJunta)
	seedAt(t, svc, "user-1", utc(2026, time.July, 1, 0), domain.TypeJunta)   // frontera End: fuera
	seedAt(t, svc, "user-2", utc(2026, time.June, 15, 10), domain.TypeJunta) // ajeno

	base := QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 6}

	// Pagina 1 de 2 (pageSize 2), orden default startsAt asc.
	p1 := base
	p1.PageSize = 2
	res, err := svc.QueryEvents(context.Background(), "user-1", p1)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.TotalItems != 3 || res.TotalPages != 2 {
		t.Fatalf("totalItems/totalPages = %d/%d, quiero 3/2", res.TotalItems, res.TotalPages)
	}
	if len(res.Events) != 2 || !res.Events[0].StartsAt.Equal(utc(2026, time.June, 3, 10)) {
		t.Fatalf("pagina 1 incorrecta: %+v", res.Events)
	}
	if res.Sort != "startsAt:asc" {
		t.Errorf("sort efectivo = %q", res.Sort)
	}

	// Filtro por tipo junta dentro de junio -> 2 (dia 3 y dia 20).
	jq := base
	jq.Type = ptr(domain.TypeJunta)
	res, _ = svc.QueryEvents(context.Background(), "user-1", jq)
	if res.TotalItems != 2 {
		t.Errorf("filtro junta: totalItems = %d, quiero 2", res.TotalItems)
	}

	// Orden desc.
	dq := base
	dq.Sort = "startsAt:desc"
	res, _ = svc.QueryEvents(context.Background(), "user-1", dq)
	if !res.Events[0].StartsAt.Equal(utc(2026, time.June, 20, 10)) {
		t.Errorf("desc: primer evento = %s", res.Events[0].StartsAt)
	}
}

func TestQueryEvents_InvalidSortAndPage(t *testing.T) {
	svc, _, _ := newEventSvc()
	if _, err := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{Sort: "nope:asc"}); !errors.Is(err, ErrValidation) {
		t.Errorf("sort invalido -> ErrValidation, obtuve %v", err)
	}
	if _, err := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{Sort: "startsAt:sideways"}); !errors.Is(err, ErrValidation) {
		t.Errorf("direccion invalida -> ErrValidation, obtuve %v", err)
	}
	if _, err := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{Page: -1}); !errors.Is(err, ErrValidation) {
		t.Errorf("page negativa -> ErrValidation, obtuve %v", err)
	}
}

func TestQueryEvents_InvalidFilterEnum(t *testing.T) {
	svc, _, _ := newEventSvc()
	badType := ptr(domain.EventType("basura"))
	if _, err := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{Type: badType}); !errors.Is(err, domain.ErrInvalidEventType) {
		t.Errorf("type fuera del enum -> ErrInvalidEventType, obtuve %v", err)
	}
	badStatus := ptr(domain.EventStatus("basura"))
	if _, err := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{Status: badStatus}); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Errorf("status fuera del enum -> ErrInvalidStatus, obtuve %v", err)
	}
	// El mismo filtro invalido debe rechazarse tambien en export.
	if _, err := svc.ExportEvents(context.Background(), "user-1", QueryEventsInput{Type: badType}); !errors.Is(err, domain.ErrInvalidEventType) {
		t.Errorf("export type invalido -> ErrInvalidEventType, obtuve %v", err)
	}
}

func TestExportEvents_AllNoPaging(t *testing.T) {
	svc, _, _ := newEventSvc()
	for i := 0; i < 5; i++ {
		seedAt(t, svc, "user-1", utc(2026, time.June, 3+i, 10), domain.TypeJunta)
	}
	res, err := svc.ExportEvents(context.Background(), "user-1", QueryEventsInput{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.Count != 5 || len(res.Events) != 5 {
		t.Errorf("count = %d, quiero 5 (sin paginar)", res.Count)
	}
	if res.SchemaVersion != "1.0" || !res.ExportedAt.Equal(fixedNow) {
		t.Errorf("metadatos de export incorrectos: %+v", res)
	}
}
