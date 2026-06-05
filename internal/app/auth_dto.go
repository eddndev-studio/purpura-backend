package app

import (
	"github.com/eddndev-studio/purpura-backend/internal/domain"
	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// RegisterInput es la entrada de Register (cuenta password). 04 seccion 5.2.
type RegisterInput struct {
	Email    string
	Nombre   string
	Password string
}

// LoginInput es la entrada de Login (cuenta password). 04 seccion 5.3.
type LoginInput struct {
	Email    string
	Password string
}

// AuthResult es la salida comun de Register/Login/AuthenticateWithGoogle. El
// handler la serializa como { accessToken, tokenType, expiresIn, user }.
type AuthResult struct {
	Token ports.IssuedToken
	User  *domain.User
}
