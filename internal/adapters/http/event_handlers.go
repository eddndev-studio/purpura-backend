package httpadapter

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eddndev-studio/purpura-backend/internal/app"
	"github.com/eddndev-studio/purpura-backend/internal/domain"
)

func (d Deps) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	e, err := d.Events.CreateEvent(r.Context(), userIDFrom(r), req.toInput())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEventResponse(*e))
}

func (d Deps) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	e, err := d.Events.GetEvent(r.Context(), userIDFrom(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventResponse(*e))
}

func (d Deps) handleQueryEvents(w http.ResponseWriter, r *http.Request) {
	in, err := parseQueryInput(r)
	if err != nil {
		writeDecodeError(w, r, err)
		return
	}
	res, err := d.Events.QueryEvents(r.Context(), userIDFrom(r), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: toEventResponses(res.Events),
		Pagination: paginationResponse{
			Page:       res.Page,
			PageSize:   res.PageSize,
			TotalItems: res.TotalItems,
			TotalPages: res.TotalPages,
			Sort:       res.Sort,
		},
	})
}

func (d Deps) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	var req updateEventRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	in := app.UpdateEventInput{Patch: req.toPatch()}
	e, err := d.Events.UpdateEvent(r.Context(), userIDFrom(r), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventResponse(*e))
}

func (d Deps) handleChangeStatus(w http.ResponseWriter, r *http.Request) {
	var req changeStatusRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	e, err := d.Events.ChangeStatus(r.Context(), userIDFrom(r), chi.URLParam(r, "id"), domain.EventStatus(req.EventStatus))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventResponse(*e))
}

func (d Deps) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := d.Events.DeleteEvent(r.Context(), userIDFrom(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) handleExportEvents(w http.ResponseWriter, r *http.Request) {
	in, err := parseQueryInput(r)
	if err != nil {
		writeDecodeError(w, r, err)
		return
	}
	res, err := d.Events.ExportEvents(r.Context(), userIDFrom(r), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="purpura-backup.json"`)
	writeJSON(w, http.StatusOK, exportResponse{
		SchemaVersion: res.SchemaVersion,
		ExportedAt:    res.ExportedAt,
		UserID:        res.UserID,
		Count:         res.Count,
		Events:        toEventResponses(res.Events),
	})
}

func (d Deps) handleImportEvents(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if req.Events == nil {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", "el campo events es obligatorio")
		return
	}
	// Modo atomic con items invalidos -> ErrValidation (422) con el arreglo de
	// errores por item; partial -> 200 con el resumen (incluye los fallos).
	sum, err := d.Events.ImportEvents(r.Context(), userIDFrom(r), req.toInput())
	if err != nil {
		if errors.Is(err, app.ErrValidation) && len(sum.Errors) > 0 {
			fields := make([]problemFieldError, len(sum.Errors))
			for i, ie := range sum.Errors {
				fields[i] = problemFieldError{Field: fmt.Sprintf("events[%d]", ie.Index), Message: ie.Detail}
			}
			writeProblemWithFields(w, r, http.StatusUnprocessableEntity, "validation_failed", err.Error(), fields)
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toImportSummaryResponse(sum))
}

// parseQueryInput extrae los parametros de consulta (04 seccion 5.7.1). Un
// numero malformado en year/month/page/pageSize -> 400 bad_request.
func parseQueryInput(r *http.Request) (app.QueryEventsInput, error) {
	q := r.URL.Query()
	in := app.QueryEventsInput{
		Mode: q.Get("mode"),
		Date: q.Get("date"),
		From: q.Get("from"),
		To:   q.Get("to"),
		TZ:   q.Get("tz"),
		Sort: q.Get("sort"),
	}
	if t := q.Get("type"); t != "" {
		et := domain.EventType(t)
		in.Type = &et
	}
	if s := q.Get("status"); s != "" {
		es := domain.EventStatus(s)
		in.Status = &es
	}

	var err error
	if in.Year, err = atoiOptional(q.Get("year")); err != nil {
		return in, errMalformedJSON
	}
	if in.Month, err = atoiOptional(q.Get("month")); err != nil {
		return in, errMalformedJSON
	}
	if in.Page, err = atoiOptional(q.Get("page")); err != nil {
		return in, errMalformedJSON
	}
	if in.PageSize, err = atoiOptional(q.Get("pageSize")); err != nil {
		return in, errMalformedJSON
	}
	return in, nil
}

func atoiOptional(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
