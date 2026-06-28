package httpadapter

import (
	"net/http"
)

// handleLinkGoogle adjunta la identidad Google del idToken a la cuenta del
// usuario autenticado (el id sale del sub del JWT, no del cuerpo). Es seguro
// porque el usuario ya esta logueado. Devuelve el usuario actualizado (con
// googleLinked=true). Conflicto (sub en otra cuenta o cuenta con otro Google) ->
// 409 google_link_conflict.
func (d Deps) handleLinkGoogle(w http.ResponseWriter, r *http.Request) {
	var req linkGoogleRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if req.IDToken == "" {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", "idToken requerido")
		return
	}
	u, err := d.Auth.LinkGoogle(r.Context(), userIDFrom(r), req.IDToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

// handleUnlinkGoogle desvincula Google de la cuenta autenticada. Si la cuenta no
// tiene credencial de contrasena (quedaria sin acceso) -> 409 cannot_unlink_google.
// Devuelve el usuario actualizado (con googleLinked=false).
func (d Deps) handleUnlinkGoogle(w http.ResponseWriter, r *http.Request) {
	u, err := d.Auth.UnlinkGoogle(r.Context(), userIDFrom(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}
