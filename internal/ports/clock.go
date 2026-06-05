package ports

import "time"

// Clock es la fuente de tiempo inyectable. Los casos de uso sellan CreatedAt y
// UpdatedAt con Now(); inyectarlo hace deterministas las pruebas (tiempo
// congelado) y mantiene "updated_at gestionado por la aplicacion".
type Clock interface {
	// Now devuelve el instante actual en UTC.
	Now() time.Time
}
