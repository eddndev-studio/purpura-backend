package httpadapter

import (
	"net/http"
	"time"
)

type healthResponse struct {
	Status  string    `json:"status"`
	Service string    `json:"service"`
	Time    time.Time `json:"time"`
}

// handleHealth responde 200 si la BD responde, 503 si no (04 seccion 5.1).
func (d Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	code := http.StatusOK
	if d.Pinger != nil {
		if err := d.Pinger.Ping(r.Context()); err != nil {
			status = "unavailable"
			code = http.StatusServiceUnavailable
		}
	}
	writeJSON(w, code, healthResponse{
		Status:  status,
		Service: "purpura-backend",
		Time:    time.Now().UTC(),
	})
}
