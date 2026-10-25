package sesion

import (
	"strings"
	"testing"
)

func TestHashClaveNoGuardaTextoPlano(t *testing.T) {
	clave := "ana-2026"
	resumen, err := HashClave(clave)
	if err != nil {
		t.Fatalf("no se pudo calcular el resumen: %v", err)
	}
	if resumen == clave || strings.Contains(resumen, clave) {
		t.Fatal("el resumen contiene la contraseña original")
	}
	if !strings.HasPrefix(resumen, "$2") {
		t.Fatalf("el resumen no parece bcrypt: %q", resumen)
	}
}

func TestClaveCoincide(t *testing.T) {
	resumen, err := HashClave("ana-2026")
	if err != nil {
		t.Fatalf("no se pudo calcular el resumen: %v", err)
	}
	if !ClaveCoincide(resumen, "ana-2026") {
		t.Fatal("la contraseña correcta no coincide")
	}
	if ClaveCoincide(resumen, "otra-clave") {
		t.Fatal("una contraseña incorrecta coincidió")
	}
	if ClaveCoincide("", "ana-2026") {
		t.Fatal("un resumen vacío coincidió")
	}
}
