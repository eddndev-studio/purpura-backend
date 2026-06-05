package domain

import "time"

// EventPatch describe una edicion parcial de un Event. Cada campo es un puntero:
// nil significa "no tocar"; presente significa "aplicar este valor". Los campos
// de contacto y ubicacion son INDEPENDIENTES (granularidad de campo, no de value
// object), de modo que un PATCH que solo trae contactName conserva el contactRef
// existente (04 seccion 5.8: "los ausentes se conservan"). Enviar "" en
// contactRef/locationLabel los limpia. El estatus NO se edita aqui: tiene su
// propio mutador ChangeStatus.
type EventPatch struct {
	Type          *EventType
	ContactName   *string
	ContactRef    *string
	LocationLat   *float64
	LocationLng   *float64
	LocationLabel *string
	Description   *string
	StartsAt      *time.Time
	Reminder      *Reminder
}

// IsEmpty indica que el patch no trae ningun campo (cuerpo PATCH vacio).
func (p EventPatch) IsEmpty() bool {
	return p.Type == nil && p.ContactName == nil && p.ContactRef == nil &&
		p.LocationLat == nil && p.LocationLng == nil && p.LocationLabel == nil &&
		p.Description == nil && p.StartsAt == nil && p.Reminder == nil
}

// Edit aplica los campos presentes del patch revalidando las invariantes
// afectadas (tipo, recordatorio, descripcion no vacia, ubicacion en rango),
// reutilizando los mismos validadores que NewEvent. Valida TODO antes de mutar:
// si alguna invariante falla, el evento no se modifica. Los campos de ubicacion
// se fusionan sobre la ubicacion actual, de modo que tocar solo uno conserva los
// demas (y la validacion de rango corre sobre el resultado fusionado).
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

	// Ubicacion candidata: la actual con los campos presentes aplicados.
	loc := e.Location
	if patch.LocationLat != nil {
		loc.Lat = *patch.LocationLat
	}
	if patch.LocationLng != nil {
		loc.Lng = *patch.LocationLng
	}
	if patch.LocationLabel != nil {
		loc.Label = *patch.LocationLabel
	}
	// Solo revalida el rango si cambiaron las coordenadas.
	if patch.LocationLat != nil || patch.LocationLng != nil {
		if err := validateLocation(loc); err != nil {
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
	if patch.StartsAt != nil {
		e.StartsAt = *patch.StartsAt
	}
	if patch.ContactName != nil {
		e.Contact.Name = *patch.ContactName
	}
	if patch.ContactRef != nil {
		e.Contact.Ref = *patch.ContactRef
	}
	e.Location = loc
	return nil
}
