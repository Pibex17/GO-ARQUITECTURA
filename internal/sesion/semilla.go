package sesion

import (
	"fmt"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/usuarios"
)

// Sembrar deja dos cuentas de demostración, una por rol. Corre en cada
// arranque: si cambian un rol o una contraseña aquí, al reiniciar el
// servidor la cuenta queda actualizada. Las contraseñas son INVENTADAS,
// solo para probar: nunca una contraseña real.
//
// Usa "usuario" como clave para no duplicar cuentas entre arranques.
func Sembrar(db *gorm.DB) error {
	ejemplos := []struct{ usuario, rol, clave string }{
		{"ana", RolCliente, "ana-2026"},
		{"luis", RolAdministrador, "luis-2026"},
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, e := range ejemplos {
			resumen, err := HashClave(e.clave)
			if err != nil {
				return fmt.Errorf("cuenta %q: %w", e.usuario, err)
			}

			var c Cuenta
			if err := tx.Where("usuario = ?", e.usuario).Limit(1).Find(&c).Error; err != nil {
				return fmt.Errorf("cuenta %q: %w", e.usuario, err)
			}
			c.Usuario = e.usuario
			c.Rol = e.rol
			c.ClaveHash = resumen
			c.UsuarioID = usuarioDeNegocio(tx, e.rol)
			if err := tx.Save(&c).Error; err != nil {
				return fmt.Errorf("cuenta %q: %w", e.usuario, err)
			}
		}
		return nil
	})
}

// usuarioDeNegocio busca el primer usuario del rol indicado. Si todavía no
// hay usuarios de negocio, la cuenta queda sin vínculo (UsuarioID nil).
func usuarioDeNegocio(db *gorm.DB, rol string) *uint {
	var u usuarios.Usuario
	if err := db.Where("rol = ?", rol).Order("id").First(&u).Error; err != nil {
		return nil
	}
	return &u.ID
}
