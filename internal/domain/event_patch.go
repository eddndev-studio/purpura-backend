package domain

import "time"

// EventPatch describe una edicion parcial de un Event. Un campo nil significa
// "no tocar". Para limpiar contact_ref o location_label se envia el Contact o
// Location completo con esa cadena vacia (semantica PATCH; ver 04 seccion 5.8).
// El estatus NO se edita aqui: tiene su propio mutador ChangeStatus.
type EventPatch struct {
	Type        *EventType
	Contact     *Contact
	Location    *Location
	Description *string
	StartsAt    *time.Time
	Reminder    *Reminder
}

// IsEmpty indica que el patch no trae ningun campo (cuerpo PATCH vacio).
func (p EventPatch) IsEmpty() bool {
	return p.Type == nil && p.Contact == nil && p.Location == nil &&
		p.Description == nil && p.StartsAt == nil && p.Reminder == nil
}

// Edit aplica los campos presentes del patch revalidando las invariantes
// afectadas (tipo, recordatorio, descripcion no vacia, ubicacion en rango),
// reutilizando los mismos validadores que NewEvent. Valida TODO antes de mutar:
// si alguna invariante falla, el evento no se modifica.
func (e *Event) Edit(patch EventPatch) error {
	if patch.Type != nil {
		if err := validateType(*patch.Type); err != nil {
			return err
		}
	}
	if patch.Reminder != nil {
		if err := validateReminder(*patch.Reminder); err != nil {
			return err
		}
	}
	var cleanDesc string
	if patch.Description != nil {
		d, err := cleanDescription(*patch.Description)
		if err != nil {
			return err
		}
		cleanDesc = d
	}
	if patch.Location != nil {
		if err := validateLocation(*patch.Location); err != nil {
			return err
		}
	}

	// Todas las invariantes pasaron: aplicar los campos presentes.
	if patch.Type != nil {
		e.Type = *patch.Type
	}
	if patch.Reminder != nil {
		e.Reminder = *patch.Reminder
	}
	if patch.Description != nil {
		e.Description = cleanDesc
	}
	if patch.Location != nil {
		e.Location = *patch.Location
	}
	if patch.Contact != nil {
		e.Contact = *patch.Contact
	}
	if patch.StartsAt != nil {
		e.StartsAt = *patch.StartsAt
	}
	return nil
}
