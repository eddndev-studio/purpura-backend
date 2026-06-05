package domain

// EventType es la categoria de un evento del organizador.
type EventType string

const (
	TypeCita            EventType = "cita"
	TypeJunta           EventType = "junta"
	TypeEntregaProyecto EventType = "entrega_proyecto"
	TypeExamen          EventType = "examen"
	TypeOtros           EventType = "otros"
)

// validEventTypes es el conjunto de tipos aceptados.
var validEventTypes = map[EventType]bool{
	TypeCita:            true,
	TypeJunta:           true,
	TypeEntregaProyecto: true,
	TypeExamen:          true,
	TypeOtros:           true,
}

// ParseEventType valida y normaliza un tipo de evento.
func ParseEventType(s string) (EventType, error) {
	t := EventType(s)
	if !validEventTypes[t] {
		return "", ErrInvalidEventType
	}
	return t, nil
}

// Valid indica si el tipo es uno de los permitidos.
func (t EventType) Valid() bool {
	return validEventTypes[t]
}
