package httpadapter

import (
	"net/http"
)

// handleMe devuelve el usuario autenticado (GET /auth/me). El id sale del sub del
// JWT (userIDFrom). Sirve para refrescar nombre/correo y el flag emailVerified
// sin re-loguear (p.ej. tras confirmar el correo en el navegador).
func (d Deps) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := d.Auth.Me(r.Context(), userIDFrom(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

// handleRequestVerification crea y envia un token de verificacion al correo del
// usuario autenticado (POST /auth/verify-email/request). Idempotente: si ya esta
// verificado, no envia nada. 202 Accepted: el envio del correo es asincrono al
// cliente. El id sale del sub del JWT, no del cuerpo.
func (d Deps) handleRequestVerification(w http.ResponseWriter, r *http.Request) {
	if err := d.Verification.RequestVerification(r.Context(), userIDFrom(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleConfirmVerification valida el token del enlace del correo y marca el
// correo como verificado (POST /auth/verify-email/confirm). Es PUBLICO: el token
// de un solo uso es la credencial. Token invalido/usado -> 400; expirado -> 410.
func (d Deps) handleConfirmVerification(w http.ResponseWriter, r *http.Request) {
	var req confirmVerificationRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if req.Token == "" {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", "token requerido")
		return
	}
	if err := d.Verification.ConfirmVerification(r.Context(), req.Token); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
