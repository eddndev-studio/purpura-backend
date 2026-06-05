package app

import (
	"fmt"
	"time"
)

// Modos de consulta temporal (04 seccion 5.7).
const (
	modeNone     = ""
	modePorDia   = "por_dia"
	modePorRango = "por_rango"
	modePorMes   = "por_mes"
	modePorAnio  = "por_anio"
)

// window es la ventana temporal semiabierta [Start, End) en UTC sobre starts_at.
// Si Has es false no se filtra por tiempo (modo "listar todos").
type window struct {
	Has   bool
	Start time.Time // inclusive, UTC
	End   time.Time // exclusive, UTC
}

// resolveWindow traduce Mode + TZ (+ date/from/to/year/month) a [Start, End) en
// UTC. EL CALENDARIO VIVE AQUI, no en el repositorio: las fronteras se calculan
// como medianoche civil en TZ y se normalizan a UTC, de modo que el repo solo
// compara starts_at contra dos instantes (coherente con 05: rango [inicio,
// siguiente), nunca extract()).
//
// Las fronteras se construyen con time.Date dejando que Go normalice el
// desbordamiento (mes 13 -> enero del ano siguiente; dia 32 -> mes siguiente),
// lo que da el "dia/mes/ano siguiente" sin aritmetica de calendario a mano. En
// dias de cambio de horario (DST) la medianoche civil sigue siendo univoca (las
// transiciones ocurren de madrugada), por lo que la ventana abarca el lapso UTC
// real del dia local (23h o 25h) de forma correcta.
func resolveWindow(in QueryEventsInput) (window, error) {
	loc, err := loadTZ(in.TZ)
	if err != nil {
		return window{}, err
	}

	switch in.Mode {
	case modeNone:
		return window{Has: false}, nil

	case modePorDia:
		d, err := parseCivilDate(in.Date)
		if err != nil {
			return window{}, err
		}
		start := time.Date(d.y, d.m, d.day, 0, 0, 0, 0, loc)
		end := time.Date(d.y, d.m, d.day+1, 0, 0, 0, 0, loc)
		return utcWindow(start, end), nil

	case modePorRango:
		from, err := parseCivilDate(in.From)
		if err != nil {
			return window{}, err
		}
		to, err := parseCivilDate(in.To)
		if err != nil {
			return window{}, err
		}
		if to.before(from) {
			return window{}, fmt.Errorf("%w: from (%s) debe ser <= to (%s)", ErrValidation, in.From, in.To)
		}
		start := time.Date(from.y, from.m, from.day, 0, 0, 0, 0, loc)
		end := time.Date(to.y, to.m, to.day+1, 0, 0, 0, 0, loc)
		return utcWindow(start, end), nil

	case modePorMes:
		if in.Year < 1 {
			return window{}, fmt.Errorf("%w: year invalido para por_mes", ErrValidation)
		}
		if in.Month < 1 || in.Month > 12 {
			return window{}, fmt.Errorf("%w: month fuera de 1..12", ErrValidation)
		}
		start := time.Date(in.Year, time.Month(in.Month), 1, 0, 0, 0, 0, loc)
		end := time.Date(in.Year, time.Month(in.Month+1), 1, 0, 0, 0, 0, loc)
		return utcWindow(start, end), nil

	case modePorAnio:
		if in.Year < 1 {
			return window{}, fmt.Errorf("%w: year invalido para por_anio", ErrValidation)
		}
		start := time.Date(in.Year, time.January, 1, 0, 0, 0, 0, loc)
		end := time.Date(in.Year+1, time.January, 1, 0, 0, 0, 0, loc)
		return utcWindow(start, end), nil

	default:
		return window{}, fmt.Errorf("%w: modo de consulta desconocido: %q", ErrValidation, in.Mode)
	}
}

func utcWindow(start, end time.Time) window {
	return window{Has: true, Start: start.UTC(), End: end.UTC()}
}

// loadTZ resuelve la zona IANA; vacia => UTC. Zona desconocida -> ErrValidation.
func loadTZ(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("%w: zona horaria invalida %q", ErrValidation, tz)
	}
	return loc, nil
}

// civilDate es una fecha sin hora ni zona (la parte de fecha de un dia).
type civilDate struct {
	y   int
	m   time.Month
	day int
}

// parseCivilDate parsea y VALIDA una fecha YYYY-MM-DD. time.Parse rechaza mes
// fuera de 1..12 y dia invalido para el mes (p.ej. 2026-02-30), de modo que la
// fecha resultante siempre existe.
func parseCivilDate(s string) (civilDate, error) {
	if s == "" {
		return civilDate{}, fmt.Errorf("%w: fecha requerida (YYYY-MM-DD)", ErrValidation)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return civilDate{}, fmt.Errorf("%w: fecha invalida %q (se espera YYYY-MM-DD)", ErrValidation, s)
	}
	return civilDate{y: t.Year(), m: t.Month(), day: t.Day()}, nil
}

// before indica si d es estrictamente anterior a o (comparacion de calendario).
func (d civilDate) before(o civilDate) bool {
	if d.y != o.y {
		return d.y < o.y
	}
	if d.m != o.m {
		return d.m < o.m
	}
	return d.day < o.day
}
