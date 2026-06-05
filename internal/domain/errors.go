package domain

import "errors"

// Errores de dominio. Los adapters los traducen a codigos HTTP.
var (
	ErrInvalidEventType  = errors.New("tipo de evento invalido")
	ErrInvalidStatus     = errors.New("estatus de evento invalido")
	ErrInvalidReminder   = errors.New("recordatorio invalido")
	ErrEmptyDescription  = errors.New("la descripcion no puede estar vacia")
	ErrInvalidLocation   = errors.New("ubicacion invalida")
	ErrEventNotFound     = errors.New("evento no encontrado")
	ErrUserNotFound      = errors.New("usuario no encontrado")
	ErrEmailTaken        = errors.New("el correo ya esta registrado")
	ErrInvalidCredential = errors.New("credenciales invalidas")
)
