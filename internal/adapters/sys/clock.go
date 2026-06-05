// Package sys agrupa los adaptadores driven triviales del sistema (reloj y
// generador de id). Implementan puertos de internal/ports y se inyectan en el
// composition root (cmd/api). Separarlos permite sustituirlos por fakes
// deterministas en pruebas.
package sys

import (
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// Clock implementa ports.Clock con la hora real del sistema en UTC.
type Clock struct{}

var _ ports.Clock = Clock{}

// NewClock construye el reloj del sistema.
func NewClock() Clock { return Clock{} }

// Now devuelve el instante actual en UTC (las marcas de tiempo del dominio y la
// BD viven en UTC; 05 seccion 4.1).
func (Clock) Now() time.Time { return time.Now().UTC() }
