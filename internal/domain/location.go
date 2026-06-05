package domain

// Location es la ubicacion de un evento, seleccionada desde el mapa.
type Location struct {
	Lat   float64
	Lng   float64
	Label string
}

// Valid verifica que las coordenadas esten dentro de rango.
func (l Location) Valid() bool {
	if l.Lat < -90 || l.Lat > 90 {
		return false
	}
	if l.Lng < -180 || l.Lng > 180 {
		return false
	}
	return true
}

// Contact es la persona con la que se tiene el evento.
// Ref guarda el identificador del contacto en la agenda del telefono.
type Contact struct {
	Name string
	Ref  string
}
