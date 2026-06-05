package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// Fakes in-memory de los puertos para probar los casos de uso de forma aislada,
// rapida y determinista (07 seccion 9.2). No usan red ni BD.

// fakeEventRepo es un EventRepository en memoria con scoping por userID.
type fakeEventRepo struct {
	store map[string]*domain.Event
	// hooks opcionales para forzar fallos de infraestructura en pruebas.
	failFindByID error
	failCreate   error
	failUpdate   error
}

var _ ports.EventRepository = (*fakeEventRepo)(nil)

func newFakeEventRepo() *fakeEventRepo {
	return &fakeEventRepo{store: map[string]*domain.Event{}}
}

func (r *fakeEventRepo) Create(_ context.Context, e *domain.Event) error {
	if r.failCreate != nil {
		return r.failCreate
	}
	cp := *e
	r.store[e.ID] = &cp
	return nil
}

func (r *fakeEventRepo) FindByID(_ context.Context, userID, id string) (*domain.Event, error) {
	if r.failFindByID != nil {
		return nil, r.failFindByID
	}
	e, ok := r.store[id]
	if !ok || e.UserID != userID {
		return nil, domain.ErrEventNotFound
	}
	cp := *e
	return &cp, nil
}

func (r *fakeEventRepo) Update(_ context.Context, e *domain.Event) error {
	if r.failUpdate != nil {
		return r.failUpdate
	}
	existing, ok := r.store[e.ID]
	if !ok || existing.UserID != e.UserID {
		return domain.ErrEventNotFound
	}
	cp := *e
	r.store[e.ID] = &cp
	return nil
}

func (r *fakeEventRepo) Delete(_ context.Context, userID, id string) error {
	e, ok := r.store[id]
	if !ok || e.UserID != userID {
		return domain.ErrEventNotFound
	}
	delete(r.store, id)
	return nil
}

func (r *fakeEventRepo) Query(_ context.Context, q ports.EventQuery) ([]domain.Event, int, error) {
	var matched []domain.Event
	for _, e := range r.store {
		if e.UserID != q.UserID {
			continue
		}
		if q.HasWindow {
			// Ventana semiabierta [Start, End): Start <= startsAt < End.
			if e.StartsAt.Before(q.Start) || !e.StartsAt.Before(q.End) {
				continue
			}
		}
		if q.Type != nil && e.Type != *q.Type {
			continue
		}
		if q.Status != nil && e.Status != *q.Status {
			continue
		}
		matched = append(matched, *e)
	}
	total := len(matched)
	sortEvents(matched, q.SortBy, q.Desc)

	lo := q.Offset
	if lo > len(matched) {
		lo = len(matched)
	}
	hi := len(matched)
	if q.Limit > 0 && lo+q.Limit < hi {
		hi = lo + q.Limit
	}
	return matched[lo:hi], total, nil
}

// sortEvents ordena por la columna pedida; ante empate, id ascendente (orden
// secundario estable de 04 seccion 6.2, que NO se invierte con desc).
func sortEvents(es []domain.Event, col string, desc bool) {
	cmp := func(a, b domain.Event) int {
		switch col {
		case "created_at":
			return cmpTime(a.CreatedAt, b.CreatedAt)
		case "updated_at":
			return cmpTime(a.UpdatedAt, b.UpdatedAt)
		case "event_type":
			return strings.Compare(string(a.Type), string(b.Type))
		case "event_status":
			return strings.Compare(string(a.Status), string(b.Status))
		default: // starts_at
			return cmpTime(a.StartsAt, b.StartsAt)
		}
	}
	sort.SliceStable(es, func(i, j int) bool {
		c := cmp(es[i], es[j])
		if c == 0 {
			return es[i].ID < es[j].ID
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
}

func cmpTime(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

// fixedClock congela el tiempo para pruebas deterministas.
type fixedClock struct{ t time.Time }

var _ ports.Clock = fixedClock{}

func (c fixedClock) Now() time.Time { return c.t }

// seqIDGen emite ids deterministas id-1, id-2, ...
type seqIDGen struct{ n int }

var _ ports.IDGenerator = (*seqIDGen)(nil)

func (g *seqIDGen) NewID() string {
	g.n++
	return fmt.Sprintf("id-%d", g.n)
}

// ptr es un helper generico para construir punteros en las tablas de prueba.
func ptr[T any](v T) *T { return &v }
