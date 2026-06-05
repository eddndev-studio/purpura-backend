package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

func validImportItem() ImportEventInput {
	return ImportEventInput{
		Type:        domain.TypeCita,
		Contact:     domain.Contact{Name: "Dr. Salinas"},
		Location:    domain.Location{Lat: 19.4, Lng: -99.1},
		Description: "Consulta medica",
		StartsAt:    utc(2026, time.June, 12, 9),
		Status:      ptr(domain.StatusPendiente),
		Reminder:    domain.ReminderOneDayBefore,
	}
}

func TestImport_PartialMixedOwnershipAndUpsert(t *testing.T) {
	svc, repo, _ := newEventSvc()
	existing, _ := svc.CreateEvent(context.Background(), "user-1", validCreate()) // id-1, CreatedAt=fixedNow

	later := utc(2026, time.June, 8, 10)
	svc.Clock = fixedClock{t: later}

	upd := validImportItem()
	upd.ID = existing.ID
	upd.Description = "Editado por import"
	upd.Status = ptr(domain.StatusRealizado)

	bad := validImportItem()
	bad.Location = domain.Location{Lat: 999, Lng: 0} // invalid_location

	in := ImportInput{
		Mode:   "partial",
		Events: []ImportEventInput{upd, validImportItem(), bad},
	}
	sum, err := svc.ImportEvents(context.Background(), "user-1", in)
	if err != nil {
		t.Fatalf("partial no debe abortar: %v", err)
	}
	if sum.Imported != 1 || sum.Updated != 1 || sum.Failed != 1 {
		t.Fatalf("summary = %+v, quiero imported=1 updated=1 failed=1", sum)
	}
	if len(sum.Errors) != 1 || sum.Errors[0].Index != 2 || sum.Errors[0].Code != "invalid_location" {
		t.Fatalf("error de item incorrecto: %+v", sum.Errors)
	}

	// El upsert conserva id y CreatedAt, re-sella UpdatedAt y respeta el status.
	got, err := repo.FindByID(context.Background(), "user-1", existing.ID)
	if err != nil {
		t.Fatalf("evento actualizado no encontrado: %v", err)
	}
	if got.Description != "Editado por import" || got.Status != domain.StatusRealizado {
		t.Errorf("upsert no aplicado: %+v", got)
	}
	if !got.CreatedAt.Equal(fixedNow) || !got.UpdatedAt.Equal(later) {
		t.Errorf("created debe conservarse y updated re-sellarse: %s / %s", got.CreatedAt, got.UpdatedAt)
	}
	if got.UserID != "user-1" {
		t.Errorf("propiedad forzada al sub fallo: %q", got.UserID)
	}
}

func TestImport_AtomicAbortsOnAnyInvalid(t *testing.T) {
	svc, repo, _ := newEventSvc()
	bad := validImportItem()
	bad.Type = domain.EventType("inexistente")

	in := ImportInput{
		Mode:   "atomic",
		Events: []ImportEventInput{validImportItem(), bad},
	}
	sum, err := svc.ImportEvents(context.Background(), "user-1", in)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("atomic con invalido -> ErrValidation, obtuve %v", err)
	}
	if len(sum.Errors) != 1 || sum.Errors[0].Code != "invalid_event_type" {
		t.Errorf("errors agregados incorrectos: %+v", sum.Errors)
	}
	// Nada debe haberse aplicado.
	res, _ := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{})
	if res.TotalItems != 0 {
		t.Errorf("atomic no debe persistir nada, hay %d", res.TotalItems)
	}
	_ = repo
}

func TestImport_PayloadStatusRespected(t *testing.T) {
	svc, _, _ := newEventSvc()
	item := validImportItem()
	item.Status = ptr(domain.StatusAplazado)
	sum, err := svc.ImportEvents(context.Background(), "user-1", ImportInput{Events: []ImportEventInput{item}})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if sum.Imported != 1 {
		t.Fatalf("summary = %+v", sum)
	}
	res, _ := svc.QueryEvents(context.Background(), "user-1", QueryEventsInput{})
	if res.Events[0].Status != domain.StatusAplazado {
		t.Errorf("status del payload no respetado: %q", res.Events[0].Status)
	}
}

func TestImport_InvalidStatusFailsItem(t *testing.T) {
	svc, _, _ := newEventSvc()
	item := validImportItem()
	item.Status = ptr(domain.EventStatus("zzz"))
	sum, _ := svc.ImportEvents(context.Background(), "user-1", ImportInput{Events: []ImportEventInput{item}})
	if sum.Failed != 1 || len(sum.Errors) != 1 || sum.Errors[0].Code != "invalid_status" {
		t.Fatalf("status invalido debe fallar el item: %+v", sum)
	}
}

func TestImport_ForeignIdCreatesNew(t *testing.T) {
	svc, repo, _ := newEventSvc()
	foreign, _ := svc.CreateEvent(context.Background(), "user-2", validCreate()) // id-1 de user-2

	item := validImportItem()
	item.ID = foreign.ID // mismo id, pero pertenece a otro usuario

	sum, err := svc.ImportEvents(context.Background(), "user-1", ImportInput{Events: []ImportEventInput{item}})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if sum.Imported != 1 || sum.Updated != 0 {
		t.Fatalf("id ajeno debe crear nuevo, no actualizar: %+v", sum)
	}
	// El evento ajeno permanece intacto.
	still, err := repo.FindByID(context.Background(), "user-2", foreign.ID)
	if err != nil || still.Description != "Revision de avance" {
		t.Errorf("el evento ajeno no debe tocarse: %+v err=%v", still, err)
	}
}
