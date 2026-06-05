package domain

import (
	"strings"
	"time"
)

// Event es un evento del organizador. Es el agregado raiz del dominio.
type Event struct {
	ID          string
	UserID      string
	Type        EventType
	Contact     Contact
	Location    Location
	Description string
	StartsAt    time.Time
	Status      EventStatus
	Reminder    Reminder
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewEvent construye un evento validando todas las invariantes del dominio.
// La fecha y la hora se combinan en StartsAt antes de llegar aqui.
func NewEvent(userID string, t EventType, contact Contact, loc Location,
	description string, startsAt time.Time, reminder Reminder) (*Event, error) {

	if err := validateType(t); err != nil {
		return nil, err
	}
	if err := validateReminder(reminder); err != nil {
		return nil, err
	}
	cleanDesc, err := cleanDescription(description)
	if err != nil {
		return nil, err
	}
	if err := validateLocation(loc); err != nil {
		return nil, err
	}

	return &Event{
		UserID:      userID,
		Type:        t,
		Contact:     contact,
		Location:    loc,
		Description: cleanDesc,
		StartsAt:    startsAt,
		Status:      StatusPendiente,
		Reminder:    reminder,
	}, nil
}

// Validadores de invariantes reutilizados por NewEvent y Edit (event_patch.go).

func validateType(t EventType) error {
	if !t.Valid() {
		return ErrInvalidEventType
	}
	return nil
}

func validateReminder(r Reminder) error {
	if !r.Valid() {
		return ErrInvalidReminder
	}
	return nil
}

func validateLocation(l Location) error {
	if !l.Valid() {
		return ErrInvalidLocation
	}
	return nil
}

// cleanDescription recorta espacios y rechaza una descripcion vacia.
func cleanDescription(description string) (string, error) {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return "", ErrEmptyDescription
	}
	return trimmed, nil
}

// ChangeStatus actualiza el estatus validando el nuevo valor.
func (e *Event) ChangeStatus(s EventStatus) error {
	if !s.Valid() {
		return ErrInvalidStatus
	}
	e.Status = s
	return nil
}

// Reschedule cambia contacto y ubicacion del evento.
func (e *Event) Reschedule(contact Contact, loc Location) error {
	if !loc.Valid() {
		return ErrInvalidLocation
	}
	e.Contact = contact
	e.Location = loc
	return nil
}
