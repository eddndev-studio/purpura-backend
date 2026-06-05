package domain

import (
	"errors"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func newValidEvent(t *testing.T) *Event {
	t.Helper()
	uid, typ, c, l, d, s, r := validArgs()
	ev, err := NewEvent(uid, typ, c, l, d, s, r)
	if err != nil {
		t.Fatalf("setup: no se esperaba error: %v", err)
	}
	return ev
}

func TestEditAppliesPresentFieldsOnly(t *testing.T) {
	ev := newValidEvent(t)
	originalStart := ev.StartsAt

	err := ev.Edit(EventPatch{
		Type:        ptr(TypeJunta),
		Description: ptr("  nueva descripcion  "),
		Reminder:    ptr(ReminderOneDayBefore),
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Type != TypeJunta {
		t.Errorf("type = %q, se esperaba junta", ev.Type)
	}
	if ev.Description != "nueva descripcion" {
		t.Errorf("description = %q, se esperaba recortada", ev.Description)
	}
	if ev.Reminder != ReminderOneDayBefore {
		t.Errorf("reminder = %q, se esperaba one_day_before", ev.Reminder)
	}
	// Campos no presentes en el patch no cambian.
	if !ev.StartsAt.Equal(originalStart) {
		t.Errorf("starts_at cambio sin estar en el patch")
	}
	if ev.Status != StatusPendiente {
		t.Errorf("Edit no debe tocar el estatus")
	}
}

func TestEditEmptyPatchIsNoOp(t *testing.T) {
	ev := newValidEvent(t)
	before := *ev
	if !(EventPatch{}).IsEmpty() {
		t.Fatalf("EventPatch{} deberia ser IsEmpty")
	}
	if err := ev.Edit(EventPatch{}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if *ev != before {
		t.Errorf("un patch vacio no debe modificar el evento")
	}
}

func TestEditRejectsInvalidAndDoesNotMutate(t *testing.T) {
	cases := []struct {
		name  string
		patch EventPatch
		want  error
	}{
		{"tipo invalido", EventPatch{Type: ptr(EventType("fiesta"))}, ErrInvalidEventType},
		{"recordatorio invalido", EventPatch{Reminder: ptr(Reminder("nunca"))}, ErrInvalidReminder},
		{"descripcion vacia", EventPatch{Description: ptr("   ")}, ErrEmptyDescription},
		{"ubicacion invalida", EventPatch{Location: ptr(Location{Lat: 200, Lng: 0})}, ErrInvalidLocation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := newValidEvent(t)
			before := *ev
			if err := ev.Edit(tc.patch); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, se esperaba %v", err, tc.want)
			}
			if *ev != before {
				t.Errorf("el evento no debe mutar cuando una invariante falla")
			}
		})
	}
}

func TestEditUpdatesContactAndLocationAndStartsAt(t *testing.T) {
	ev := newValidEvent(t)
	newStart := time.Date(2027, 1, 2, 9, 30, 0, 0, time.UTC)
	newLoc := Location{Lat: 40.4168, Lng: -3.7038, Label: "Madrid"}
	newContact := Contact{Name: "Ana", Ref: ""}

	err := ev.Edit(EventPatch{
		Contact:  &newContact,
		Location: &newLoc,
		StartsAt: &newStart,
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Contact != newContact {
		t.Errorf("contact = %+v, se esperaba %+v", ev.Contact, newContact)
	}
	if ev.Location != newLoc {
		t.Errorf("location = %+v, se esperaba %+v", ev.Location, newLoc)
	}
	if !ev.StartsAt.Equal(newStart) {
		t.Errorf("starts_at = %v, se esperaba %v", ev.StartsAt, newStart)
	}
}
