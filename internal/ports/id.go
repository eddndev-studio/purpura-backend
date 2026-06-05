package ports

// IDGenerator genera identificadores unicos. Los casos de uso asignan el id de
// las entidades nuevas antes de persistir; inyectarlo permite ids deterministas
// en pruebas.
type IDGenerator interface {
	// NewID devuelve un identificador unico nuevo (UUID v4 como string).
	NewID() string
}
