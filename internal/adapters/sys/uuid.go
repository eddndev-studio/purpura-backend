package sys

import (
	"github.com/google/uuid"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// UUIDGenerator implementa ports.IDGenerator con UUID v4.
type UUIDGenerator struct{}

var _ ports.IDGenerator = UUIDGenerator{}

// NewUUIDGenerator construye el generador de ids del sistema.
func NewUUIDGenerator() UUIDGenerator { return UUIDGenerator{} }

// NewID devuelve un UUID v4 como string (la PK uuid de events/users; 05).
func (UUIDGenerator) NewID() string { return uuid.NewString() }
