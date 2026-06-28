package domain

import "errors"

// Errores de dominio. Los adapters los traducen a codigos HTTP.
var (
	ErrInvalidEventType    = errors.New("tipo de evento invalido")
	ErrInvalidStatus       = errors.New("estatus de evento invalido")
	ErrInvalidReminder     = errors.New("recordatorio invalido")
	ErrEmptyDescription    = errors.New("la descripcion no puede estar vacia")
	ErrInvalidLocation     = errors.New("ubicacion invalida")
	ErrEventNotFound       = errors.New("evento no encontrado")
	ErrUserNotFound        = errors.New("usuario no encontrado")
	ErrEmailTaken          = errors.New("el correo ya esta registrado")
	ErrInvalidCredential   = errors.New("credenciales invalidas")
	ErrInvalidEmail        = errors.New("correo invalido")
	ErrEmptyName           = errors.New("el nombre no puede estar vacio")
	ErrInvalidAuthProvider = errors.New("proveedor de autenticacion invalido")
	// ErrGoogleLinkConflict: no se puede vincular Google porque esa identidad ya
	// esta en otra cuenta, o esta cuenta ya tiene un Google distinto adjunto.
	ErrGoogleLinkConflict = errors.New("conflicto al vincular Google")
	// ErrCannotUnlinkGoogle: desvincular dejaria la cuenta sin ningun metodo de
	// inicio de sesion (no tiene credencial de contrasena).
	ErrCannotUnlinkGoogle = errors.New("no se puede desvincular Google: la cuenta quedaria sin acceso")
)
