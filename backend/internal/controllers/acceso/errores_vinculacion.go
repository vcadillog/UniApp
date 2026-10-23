package acceso

import (
	"errors"
	"net/http"

	modelo "uniapp/internal/models/acceso"
)

func registrarErroresDeVinculacion(m *MapeadorDeErrores) {
	m.Registrar(func(err error) bool {
		return esAlguno(err, modelo.ErrNoEsApoderado, modelo.ErrNoEsEscolar)
	}, respuestaConEstado(http.StatusForbidden, "rol_no_permitido"))
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrEscolarNoEncontrado) },
		respuestaConEstado(http.StatusNotFound, "escolar_no_encontrado"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrEscolarNoValido) },
		respuestaConCampos(http.StatusUnprocessableEntity, "escolar_no_valido", "email_escolar"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrYaVinculado) },
		respuestaConEstado(http.StatusConflict, "escolar_ya_vinculado"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSolicitudPendiente) },
		respuestaConEstado(http.StatusConflict, "solicitud_pendiente"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSinApoderado) },
		respuestaConEstado(http.StatusConflict, "sin_apoderado"),
	)
	m.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrVinculacionAjena) },
		respuestaConEstado(http.StatusForbidden, "no_autorizado"),
	)
}
