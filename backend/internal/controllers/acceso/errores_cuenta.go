package acceso

import (
	"errors"
	"net/http"

	modelo "uniapp/internal/models/acceso"
	vista "uniapp/internal/views/acceso"
)

func registrarErroresDeCuenta(m *MapeadorDeErrores) {
	m.Registrar(func(err error) bool {
		var validacion modelo.ErrValidacion
		return errors.As(err, &validacion)
	}, func(err error) vista.RespuestaError {
		var validacion modelo.ErrValidacion
		_ = errors.As(err, &validacion)
		return vista.RespuestaError{
			Estado: http.StatusUnprocessableEntity, Codigo: "validacion",
			Mensaje: "Revisa los datos ingresados.", Campos: validacion.Campos,
		}
	})
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrEmailEnUso) },
		respuestaConCampos(http.StatusConflict, "correo_en_uso", "email"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrRolNoPermitido) },
		respuestaConEstado(http.StatusForbidden, "rol_no_permitido"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrCredenciales) },
		respuestaConEstado(http.StatusUnauthorized, "credenciales"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrDemasiadosIntentos) },
		respuestaConEstado(http.StatusTooManyRequests, "demasiados_intentos"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrTokenInvalido) },
		respuestaConEstado(http.StatusBadRequest, "token_invalido"),
	)
}
