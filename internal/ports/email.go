package ports

import "context"

// VerificationEmail son los datos que el adaptador de correo necesita para armar
// y enviar el mensaje de verificacion. VerifyURL ya incluye el token crudo.
type VerificationEmail struct {
	To        string
	Nombre    string
	VerifyURL string
}

// EmailSender envia correos transaccionales. La implementacion de produccion usa
// Resend; cuando no hay API key, un adaptador que solo registra el enlace (log)
// la sustituye para no bloquear el despliegue.
type EmailSender interface {
	// SendVerificationEmail envia el correo con el enlace de verificacion. Un
	// fallo (red, API) se propaga: el caso de uso decide como tratarlo.
	SendVerificationEmail(ctx context.Context, msg VerificationEmail) error
}
