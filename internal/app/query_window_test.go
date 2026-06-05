package app

import (
	"errors"
	"testing"
	"time"
)

func utc(y int, mo time.Month, d, h int) time.Time {
	return time.Date(y, mo, d, h, 0, 0, 0, time.UTC)
}

func TestResolveWindow_Valid(t *testing.T) {
	cases := []struct {
		name      string
		in        QueryEventsInput
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "por_dia UTC",
			in:        QueryEventsInput{Mode: modePorDia, Date: "2026-06-10"},
			wantStart: utc(2026, time.June, 10, 0),
			wantEnd:   utc(2026, time.June, 11, 0),
		},
		{
			// DST spring-forward en una zona que SI observa DST. NY: 2026-03-08
			// 00:00 sigue en EST (UTC-5 -> 05:00Z); 03-09 00:00 ya es EDT
			// (UTC-4 -> 04:00Z). La ventana abarca 23h reales.
			name:      "por_dia DST spring-forward New_York",
			in:        QueryEventsInput{Mode: modePorDia, Date: "2026-03-08", TZ: "America/New_York"},
			wantStart: utc(2026, time.March, 8, 5),
			wantEnd:   utc(2026, time.March, 9, 4),
		},
		{
			name:      "por_rango from==to es un solo dia",
			in:        QueryEventsInput{Mode: modePorRango, From: "2026-06-10", To: "2026-06-10"},
			wantStart: utc(2026, time.June, 10, 0),
			wantEnd:   utc(2026, time.June, 11, 0),
		},
		{
			name:      "por_rango varios dias inclusivo en to",
			in:        QueryEventsInput{Mode: modePorRango, From: "2026-06-05", To: "2026-06-09"},
			wantStart: utc(2026, time.June, 5, 0),
			wantEnd:   utc(2026, time.June, 10, 0),
		},
		{
			name:      "por_mes febrero no bisiesto 2026",
			in:        QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 2},
			wantStart: utc(2026, time.February, 1, 0),
			wantEnd:   utc(2026, time.March, 1, 0),
		},
		{
			name:      "por_mes febrero bisiesto 2024",
			in:        QueryEventsInput{Mode: modePorMes, Year: 2024, Month: 2},
			wantStart: utc(2024, time.February, 1, 0),
			wantEnd:   utc(2024, time.March, 1, 0),
		},
		{
			// month+1 -> 13 debe normalizar a enero del ano siguiente.
			name:      "por_mes diciembre desborda a enero siguiente",
			in:        QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 12},
			wantStart: utc(2026, time.December, 1, 0),
			wantEnd:   utc(2027, time.January, 1, 0),
		},
		{
			name:      "por_anio 2026",
			in:        QueryEventsInput{Mode: modePorAnio, Year: 2026},
			wantStart: utc(2026, time.January, 1, 0),
			wantEnd:   utc(2027, time.January, 1, 0),
		},
		{
			// por_mes en tz con offset: 2026-06-01 00:00 EDT (UTC-4) = 04:00Z.
			name:      "por_mes en America/New_York",
			in:        QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 6, TZ: "America/New_York"},
			wantStart: utc(2026, time.June, 1, 4),
			wantEnd:   utc(2026, time.July, 1, 4),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := resolveWindow(tc.in)
			if err != nil {
				t.Fatalf("resolveWindow error inesperado: %v", err)
			}
			if !w.Has {
				t.Fatalf("esperaba ventana activa (Has=true)")
			}
			if !w.Start.Equal(tc.wantStart) {
				t.Errorf("Start = %s, quiero %s", w.Start.Format(time.RFC3339), tc.wantStart.Format(time.RFC3339))
			}
			if !w.End.Equal(tc.wantEnd) {
				t.Errorf("End = %s, quiero %s", w.End.Format(time.RFC3339), tc.wantEnd.Format(time.RFC3339))
			}
			if w.Start.Location() != time.UTC || w.End.Location() != time.UTC {
				t.Errorf("las fronteras deben estar en UTC")
			}
		})
	}
}

func TestResolveWindow_NoMode(t *testing.T) {
	w, err := resolveWindow(QueryEventsInput{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if w.Has {
		t.Errorf("sin mode no debe haber ventana temporal")
	}
}

func TestResolveWindow_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   QueryEventsInput
	}{
		{"modo desconocido", QueryEventsInput{Mode: "por_semana"}},
		{"por_dia sin date", QueryEventsInput{Mode: modePorDia}},
		{"por_dia fecha inexistente", QueryEventsInput{Mode: modePorDia, Date: "2026-02-30"}},
		{"por_dia formato malo", QueryEventsInput{Mode: modePorDia, Date: "10-06-2026"}},
		{"por_rango from>to", QueryEventsInput{Mode: modePorRango, From: "2026-06-10", To: "2026-06-05"}},
		{"por_rango sin from", QueryEventsInput{Mode: modePorRango, To: "2026-06-05"}},
		{"por_mes month 13", QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 13}},
		{"por_mes month 0", QueryEventsInput{Mode: modePorMes, Year: 2026, Month: 0}},
		{"por_mes year invalido", QueryEventsInput{Mode: modePorMes, Year: 0, Month: 6}},
		{"por_anio year invalido", QueryEventsInput{Mode: modePorAnio, Year: 0}},
		{"tz invalida", QueryEventsInput{Mode: modePorDia, Date: "2026-06-10", TZ: "Marte/Olympus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveWindow(tc.in)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("quiero ErrValidation, obtuve %v", err)
			}
		})
	}
}
