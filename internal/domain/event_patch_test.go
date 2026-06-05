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
		{"latitud fuera de rango", EventPatch{LocationLat: ptr(200.0)}, ErrInvalidLocation},
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

	err := ev.Edit(EventPatch{
		ContactName:   ptr("Ana"),
		ContactRef:    ptr(""),
		LocationLat:   ptr(40.4168),
		LocationLng:   ptr(-3.7038),
		LocationLabel: ptr("Madrid"),
		StartsAt:      &newStart,
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Contact != (Contact{Name: "Ana", Ref: ""}) {
		t.Errorf("contact = %+v", ev.Contact)
	}
	if ev.Location != (Location{Lat: 40.4168, Lng: -3.7038, Label: "Madrid"}) {
		t.Errorf("location = %+v", ev.Location)
	}
	if !ev.StartsAt.Equal(newStart) {
		t.Errorf("starts_at = %v, se esperaba %v", ev.StartsAt, newStart)
	}
}

// Regresion (hallazgo de revision): un PATCH parcial de un solo subcampo de
// contacto o ubicacion debe CONSERVAR los hermanos no enviados (04 seccion 5.8).
func TestEditPartialPreservesSiblingFields(t *testing.T) {
	ev := newValidEvent(t)
	ev.Contact = Contact{Name: "Original", Ref: "ref-original"}
	ev.Location = Location{Lat: 19.43, Lng: -99.13, Label: "CDMX"}

	// Solo cambia contactName: contactRef debe permanecer.
	if err := ev.Edit(EventPatch{ContactName: ptr("Nuevo")}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if ev.Contact.Name != "Nuevo" || ev.Contact.Ref != "ref-original" {
		t.Errorf("contact = %+v, se esperaba conservar Ref", ev.Contact)
	}

	// Solo cambia locationLabel: lat/lng deben permanecer.
	if err := ev.Edit(EventPatch{LocationLabel: ptr("Centro")}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if ev.Location.Lat != 19.43 || ev.Location.Lng != -99.13 || ev.Location.Label != "Centro" {
		t.Errorf("location = %+v, se esperaba conservar lat/lng", ev.Location)
	}
}
