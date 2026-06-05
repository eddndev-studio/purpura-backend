package ports

import "context"

// PasswordHasher deriva y verifica hashes de contrasena. Existe como puerto para
// que los casos de uso (Register/Login) no se acoplen a la libreria de hashing.
type PasswordHasher interface {
	// Hash deriva un hash seguro (p.ej. bcrypt) de la contrasena en claro.
	Hash(ctx context.Context, plain string) (string, error)

	// Compare verifica una contrasena contra su hash. Si no coincide:
	// domain.ErrInvalidCredential. Otros fallos: error de infraestructura.
	Compare(ctx context.Context, hash, plain string) error
}
