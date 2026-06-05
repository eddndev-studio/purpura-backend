// Package arch contiene la prueba de arquitectura (07 seccion 9.6): verifica la
// regla de dependencia del hexagono inspeccionando los imports de cada paquete.
// Falla si el dominio importa algo del proyecto, si app importa un adaptador, o
// si un adaptador importa otro. Cementa la forma del hexagono en CI.
package arch

import (
	"go/build"
	"strings"
	"testing"
)

const modulePrefix = "github.com/eddndev-studio/purpura-backend/"

// projectImports devuelve los imports internos del proyecto (sin stdlib ni
// dependencias externas) del paquete en dir (relativo a internal/arch).
func projectImports(t *testing.T, dir string) []string {
	t.Helper()
	pkg, err := build.ImportDir(dir, 0)
	if err != nil {
		t.Fatalf("no se pudo analizar %s: %v", dir, err)
	}
	var out []string
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp, modulePrefix) {
			out = append(out, strings.TrimPrefix(imp, modulePrefix))
		}
	}
	return out
}

func assertAllowed(t *testing.T, dir string, allowed ...string) {
	t.Helper()
	allow := map[string]bool{}
	for _, a := range allowed {
		allow[a] = true
	}
	for _, imp := range projectImports(t, dir) {
		if !allow[imp] {
			t.Errorf("%s importa %q, que viola la regla de dependencia", dir, imp)
		}
	}
}

func TestDomainHasNoProjectImports(t *testing.T) {
	// El nucleo no conoce nada del proyecto (solo stdlib).
	assertAllowed(t, "../domain")
}

func TestPortsImportOnlyDomain(t *testing.T) {
	assertAllowed(t, "../ports", "internal/domain")
}

func TestAppImportsOnlyDomainAndPorts(t *testing.T) {
	assertAllowed(t, "../app", "internal/domain", "internal/ports")
}

func TestAdaptersDoNotImportEachOther(t *testing.T) {
	// Cada adaptador puede usar domain, ports, app y (postgres) el paquete db
	// generado, pero NUNCA otro adaptador.
	cases := map[string][]string{
		"../adapters/http": {
			"internal/domain", "internal/ports", "internal/app",
		},
		"../adapters/postgres": {
			"internal/domain", "internal/ports", "internal/db",
		},
		"../adapters/auth": {
			"internal/domain", "internal/ports",
		},
		"../adapters/sys": {
			"internal/ports",
		},
	}
	for dir, allowed := range cases {
		assertAllowed(t, dir, allowed...)
	}
}
