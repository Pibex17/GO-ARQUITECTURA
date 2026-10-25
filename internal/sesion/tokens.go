package sesion

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Duracion es cuánto vale un token desde que se emite.
const Duracion = time.Hour

// Sesion es lo que el servidor sabe de quien hace la petición.
// Sale del token: no se consulta la base de datos en cada petición.
type Sesion struct {
	CuentaID uint
	Usuario  string
	Rol      string
}

// datos es lo que viaja dentro del token. Cualquiera puede leerlo:
// va codificado, no cifrado. Lo que nadie puede hacer es cambiarlo.
type datos struct {
	Usuario string `json:"usuario"`
	Rol     string `json:"rol"`
	jwt.RegisteredClaims
}

// Tokens emite y revisa tokens con el secreto que recibe.
type Tokens struct {
	secreto []byte
}

// NuevosTokens recibe el secreto de la firma. El secreto llega desde la
// configuración (la variable de entorno); nunca se escribe en el código.
func NuevosTokens(secreto string) *Tokens {
	return &Tokens{secreto: []byte(secreto)}
}

// Emitir firma un token para la cuenta que acaba de probar quién es.
func (t *Tokens) Emitir(c Cuenta) (string, error) {
	ahora := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, datos{
		Usuario: c.Usuario,
		Rol:     c.Rol,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(c.ID), 10),
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(ahora.Add(Duracion)),
		},
	})
	return token.SignedString(t.secreto)
}

// ErrVencido distingue un token que fue bueno de uno que nunca lo fue.
var ErrVencido = errors.New("el token está vencido")

// ErrAlgoritmo indica que el token no fue firmado con HS256.
var ErrAlgoritmo = errors.New("el token no usa HS256")

// Leer revisa el algoritmo, la firma y la fecha de vencimiento.
// Solo si todo pasa devuelve la sesión.
func (t *Tokens) Leer(texto string) (Sesion, error) {
	var d datos
	_, err := jwt.ParseWithClaims(texto, &d,
		func(token *jwt.Token) (any, error) {
			// Comprobación explícita además de WithValidMethods: durante la
			// validación solo se acepta HS256, nunca "none" ni otro algoritmo.
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrAlgoritmo
			}
			return t.secreto, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if errors.Is(err, jwt.ErrTokenExpired) {
		return Sesion{}, ErrVencido
	}
	if err != nil {
		return Sesion{}, err
	}

	id, err := strconv.ParseUint(d.Subject, 10, 64)
	if err != nil || id == 0 {
		return Sesion{}, errors.New("el token no identifica una cuenta")
	}
	return Sesion{CuentaID: uint(id), Usuario: d.Usuario, Rol: d.Rol}, nil
}
