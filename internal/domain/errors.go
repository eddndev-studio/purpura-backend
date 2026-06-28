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
	// ErrEmailNotVerified: el idToken trae email_verified=false. No se puede
	// confiar en ese correo para crear ni reconciliar una cuenta por email (el
	// correo podria pertenecer a otra persona), asi que se rechaza el login.
	ErrEmailNotVerified = errors.New("el correo de Google no esta verificado")
	// ErrInvalidGoogleToken: el idToken de Google es invalido como PARAMETRO de
	// una operacion ya autenticada (p.ej. vincular). Se distingue de la falta de
	// sesion (401) para que el cliente no confunda "sesion expirada" con "token
	// de Google rechazado".
	ErrInvalidGoogleToken = errors.New("idToken de Google invalido")
	// ErrInvalidVerificationToken: el token de verificacion de correo no existe o
	// ya fue usado. No se distingue "inexistente" de "usado" para no filtrar la
	// existencia de tokens.
	ErrInvalidVerificationToken = errors.New("token de verificacion invalido")
	// ErrVerificationTokenExpired: el token existe pero ya expiro; el usuario debe
	// solicitar uno nuevo.
	ErrVerificationTokenExpired = errors.New("token de verificacion expirado")
)
