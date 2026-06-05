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

	if !t.Valid() {
		return nil, ErrInvalidEventType
	}
	if !reminder.Valid() {
		return nil, ErrInvalidReminder
	}
	if strings.TrimSpace(description) == "" {
		return nil, ErrEmptyDescription
	}
	if !loc.Valid() {
		return nil, ErrInvalidLocation
	}

	return &Event{
		UserID:      userID,
		Type:        t,
		Contact:     contact,
		Location:    loc,
		Description: strings.TrimSpace(description),
		StartsAt:    startsAt,
		Status:      StatusPendiente,
		Reminder:    reminder,
	}, nil
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
