package httpadapter

import (
	"net/http"

	"github.com/eddndev-studio/purpura-backend/internal/app"
)

func (d Deps) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	res, err := d.Auth.Register(r.Context(), app.RegisterInput{
		Email:    req.Email,
		Nombre:   req.Nombre,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAuthResponse(res))
}

func (d Deps) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	res, err := d.Auth.Login(r.Context(), app.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(res))
}

func (d Deps) handleGoogle(w http.ResponseWriter, r *http.Request) {
	var req googleRequest
	if err := decodeJSON(w, r, d.MaxBodyBytes, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if req.IDToken == "" {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", "idToken requerido")
		return
	}
	res, err := d.Auth.AuthenticateWithGoogle(r.Context(), req.IDToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(res))
}

// handleDeleteAccount elimina permanentemente la cuenta del usuario autenticado y
// todos sus datos (cascada en BD). El id sale del sub del JWT (userIDFrom), nunca
// del cuerpo: un usuario solo puede borrarse a si mismo. 204 sin cuerpo en exito.
func (d Deps) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if err := d.Auth.DeleteAccount(r.Context(), userIDFrom(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
