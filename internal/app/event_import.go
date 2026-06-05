package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

// Modos de aplicacion de import (04 seccion 5.12).
const (
	importModePartial = "partial"
	importModeAtomic  = "atomic"
)

// preparedImport es un evento ya reconstruido y validado, listo para persistir.
type preparedImport struct {
	event    *domain.Event
	isUpdate bool
}

// ImportEvents restaura eventos desde un documento de respaldo hacia la cuenta
// del usuario autenticado. A diferencia de CreateEvent, respeta el eventStatus
// del payload y aplica la politica de id de 04 seccion 5.12.
//
// Disciplina: PRIMERO se validan/reconstruyen todos los eventos; solo despues se
// persisten. En modo atomic, si algun item es invalido no se aplica ninguno
// (atomicidad de validacion). En modo partial se aplican los validos y se
// reportan los invalidos en Errors[], sin abortar.
func (s *EventService) ImportEvents(ctx context.Context, userID string, in ImportInput) (ImportSummary, error) {
	mode := in.Mode
	if mode == "" {
		mode = importModePartial
	}
	if mode != importModePartial && mode != importModeAtomic {
		return ImportSummary{}, fmt.Errorf("%w: modo de import invalido: %q", ErrValidation, mode)
	}

	var (
		summary  ImportSummary
		prepared []preparedImport
		failures []ImportItemError
	)

	for i, item := range in.Events {
		e, ferr := s.reconstructImport(ctx, userID, item)
		if ferr != nil {
			// Un error de infraestructura (no de validacion del item) aborta:
			// no es un item invalido, es un fallo de lectura del repositorio.
			var infra *importInfraError
			if errors.As(ferr, &infra) {
				return ImportSummary{}, infra.err
			}
			failures = append(failures, ImportItemError{
				Index:  i,
				Code:   ErrorCode(ferr),
				Detail: ferr.Error(),
			})
			continue
		}
		prepared = append(prepared, *e)
	}

	// Modo atomic: cualquier item invalido aborta todo antes de tocar el repo.
	if mode == importModeAtomic && len(failures) > 0 {
		return ImportSummary{Errors: failures}, fmt.Errorf("%w: import atomico fallido: %d evento(s) invalido(s)", ErrValidation, len(failures))
	}

	// Aplicar los preparados (en partial, los validos; en atomic, todos).
	for _, p := range prepared {
		if p.isUpdate {
			if err := s.Events.Update(ctx, p.event); err != nil {
				return ImportSummary{}, err
			}
			summary.Updated++
			continue
		}
		if err := s.Events.Create(ctx, p.event); err != nil {
			return ImportSummary{}, err
		}
		summary.Imported++
	}

	summary.Failed = len(failures)
	summary.Errors = failures
	return summary, nil
}

// importInfraError envuelve un fallo de infraestructura para distinguirlo de un
// item invalido: el primero aborta el import; el segundo se acumula en Errors[].
type importInfraError struct{ err error }

func (e *importInfraError) Error() string { return e.err.Error() }

// reconstructImport valida y reconstruye un evento del payload sin persistirlo.
// Aplica la politica de id (04 seccion 5.12): id propio existente -> Update
// conservando CreatedAt; id ausente/vacio/ajeno -> Create con id nuevo.
func (s *EventService) reconstructImport(ctx context.Context, userID string, item ImportEventInput) (*preparedImport, error) {
	e, err := domain.NewEvent(userID, item.Type, item.Contact, item.Location, item.Description, item.StartsAt, item.Reminder)
	if err != nil {
		return nil, err
	}
	// eventStatus del payload SI se respeta (la restauracion preserva el estado).
	if item.Status != nil {
		if err := e.ChangeStatus(*item.Status); err != nil {
			return nil, err
		}
	}
	// Propiedad forzada al sub: se ignora cualquier userId del payload.
	e.UserID = userID

	if item.ID != "" {
		existing, err := s.Events.FindByID(ctx, userID, item.ID)
		switch {
		case err == nil:
			// Existe y es propio -> actualizacion: conservar id y CreatedAt.
			e.ID = existing.ID
			e.CreatedAt = existing.CreatedAt
			e.UpdatedAt = s.Clock.Now()
			return &preparedImport{event: e, isUpdate: true}, nil
		case errors.Is(err, domain.ErrEventNotFound):
			// No existe o pertenece a OTRO usuario: se crea con id nuevo (abajo).
		default:
			// Fallo de infraestructura: aborta el import, no es item invalido.
			return nil, &importInfraError{err: err}
		}
	}

	// Creacion con id nuevo del backend (nunca se reutiliza el id del payload).
	e.ID = s.IDs.NewID()
	now := s.Clock.Now()
	e.CreatedAt = now
	e.UpdatedAt = now
	return &preparedImport{event: e, isUpdate: false}, nil
}
