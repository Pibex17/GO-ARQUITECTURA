package sesion

import "golang.org/x/crypto/bcrypt"

// HashClave calcula el resumen bcrypt que se guarda en la cuenta.
// La contraseña original no se guarda en ningún lado.
func HashClave(clave string) (string, error) {
	resumen, err := bcrypt.GenerateFromPassword([]byte(clave), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(resumen), nil
}

// ClaveCoincide compara una contraseña con el resumen guardado.
func ClaveCoincide(resumen, clave string) bool {
	return bcrypt.CompareHashAndPassword([]byte(resumen), []byte(clave)) == nil
}
