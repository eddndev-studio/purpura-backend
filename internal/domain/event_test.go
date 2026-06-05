package domain

import (
	"errors"
	"testing"
	"time"
)

func validArgs() (string, EventType, Contact, Location, string, time.Time, Reminder) {
	return "user-1",
		TypeCita,
		Contact{Name: "Alejandro", Ref: "contact-7"},
		Location{Lat: 19.4326, Lng: -99.1332, Label: "CDMX"},
		"Cita para comer",
		time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		ReminderTenMinutes
}

func TestNewEventDefaultsToPendiente(t *testing.T) {
	uid, typ, c, l, d, s, r := validArgs()
	ev, err := NewEvent(uid, typ, c, l, d, s, r)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Status != StatusPendiente {
		t.Fatalf("estatus inicial = %q, se esperaba %q", ev.Status, StatusPendiente)
	}
}

func TestNewEventTrimsDescription(t *testing.T) {
	uid, typ, c, l, _, s, r := validArgs()
	ev, err := NewEvent(uid, typ, c, l, "  reunion  ", s, r)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Description != "reunion" {
		t.Fatalf("descripcion = %q, se esperaba %q", ev.Description, "reunion")
	}
}

func TestNewEventRejectsInvalidInput(t *testing.T) {
	uid, typ, c, l, d, s, r := validArgs()

	if _, err := NewEvent(uid, "fiesta", c, l, d, s, r); !errors.Is(err, ErrInvalidEventType) {
		t.Errorf("tipo invalido: err = %v, se esperaba ErrInvalidEventType", err)
	}
	if _, err := NewEvent(uid, typ, c, l, "   ", s, r); !errors.Is(err, ErrEmptyDescription) {
		t.Errorf("descripcion vacia: err = %v, se esperaba ErrEmptyDescription", err)
	}
	if _, err := NewEvent(uid, typ, c, l, d, s, "nunca"); !errors.Is(err, ErrInvalidReminder) {
		t.Errorf("recordatorio invalido: err = %v, se esperaba ErrInvalidReminder", err)
	}
	bad := Location{Lat: 200, Lng: 0}
	if _, err := NewEvent(uid, typ, c, bad, d, s, r); !errors.Is(err, ErrInvalidLocation) {
		t.Errorf("ubicacion invalida: err = %v, se esperaba ErrInvalidLocation", err)
	}
}

func TestChangeStatus(t *testing.T) {
	uid, typ, c, l, d, s, r := validArgs()
	ev, _ := NewEvent(uid, typ, c, l, d, s, r)

	if err := ev.ChangeStatus(StatusRealizado); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if ev.Status != StatusRealizado {
		t.Fatalf("estatus = %q, se esperaba %q", ev.Status, StatusRealizado)
	}
	if err := ev.ChangeStatus("cancelado"); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("estatus invalido: err = %v, se esperaba ErrInvalidStatus", err)
	}
}

func TestRescheduleRejectsBadLocation(t *testing.T) {
	uid, typ, c, l, d, s, r := validArgs()
	ev, _ := NewEvent(uid, typ, c, l, d, s, r)

	err := ev.Reschedule(Contact{Name: "Ana"}, Location{Lat: 0, Lng: 999})
	if !errors.Is(err, ErrInvalidLocation) {
		t.Errorf("err = %v, se esperaba ErrInvalidLocation", err)
	}
}
