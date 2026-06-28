package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/eddndev-studio/purpura-backend/internal/ports"
)

// resendEndpoint es la API de envio de Resend (https://resend.com/docs).
const resendEndpoint = "https://api.resend.com/emails"

// ResendSender envia correos transaccionales via Resend. La API key es un secreto
// (Bearer); from es el remitente verificado (p.ej. noreply@purpura.eddn.dev).
type ResendSender struct {
	apiKey   string
	from     string
	client   *http.Client
	endpoint string // inyectable para pruebas; vacio = resendEndpoint
}

var _ ports.EmailSender = (*ResendSender)(nil)

// NewResendSender construye el sender con timeout de red acotado.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// SendVerificationEmail arma y envia el correo de verificacion. Un status >= 300
// se trata como error (con un fragmento del cuerpo para diagnostico).
func (s *ResendSender) SendVerificationEmail(ctx context.Context, msg ports.VerificationEmail) error {
	payload := resendRequest{
		From:    s.from,
		To:      []string{msg.To},
		Subject: "Verifica tu correo en Purpura",
		HTML:    verificationHTML(msg),
		Text:    verificationText(msg),
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal resend request: %w", err)
	}
	endpoint := s.endpoint
	if endpoint == "" {
		endpoint = resendEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("email: new resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("email: resend send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("email: resend respondio %d: %s", resp.StatusCode, string(snippet))
	}
	return nil
}

// verificationText es la version en texto plano (clientes sin HTML). El nombre y
// la URL se interpolan tal cual (la URL la genera el backend, no el usuario).
func verificationText(msg ports.VerificationEmail) string {
	return fmt.Sprintf(
		"Hola %s,\n\nConfirma tu correo en Purpura abriendo este enlace:\n%s\n\n"+
			"Si no creaste una cuenta, ignora este mensaje.",
		msg.Nombre, msg.VerifyURL)
}

// verificationHTML escapa el nombre (dato del usuario) para evitar inyeccion en
// el correo; la URL la controla el backend.
func verificationHTML(msg ports.VerificationEmail) string {
	return fmt.Sprintf(
		`<p>Hola %s,</p>`+
			`<p>Confirma tu correo en Purpura:</p>`+
			`<p><a href="%s">Verificar mi correo</a></p>`+
			`<p>Si no creaste una cuenta, ignora este mensaje.</p>`,
		html.EscapeString(msg.Nombre), html.EscapeString(msg.VerifyURL))
}
