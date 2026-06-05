package domain

// EventStatus es el estado de avance de un evento.
type EventStatus string

const (
	StatusPendiente EventStatus = "pendiente"
	StatusRealizado EventStatus = "realizado"
	StatusAplazado  EventStatus = "aplazado"
)

var validStatuses = map[EventStatus]bool{
	StatusPendiente: true,
	StatusRealizado: true,
	StatusAplazado:  true,
}

// ParseEventStatus valida y normaliza un estatus.
func ParseEventStatus(s string) (EventStatus, error) {
	st := EventStatus(s)
	if !validStatuses[st] {
		return "", ErrInvalidStatus
	}
	return st, nil
}

// Valid indica si el estatus es uno de los permitidos.
func (s EventStatus) Valid() bool {
	return validStatuses[s]
}
