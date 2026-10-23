package reserva

import (
	"errors"
	"net/http"

	ctrlAcceso "uniapp/internal/controllers/acceso"
	modelo "uniapp/internal/models/reserva"
	vistaAcceso "uniapp/internal/views/acceso"
)

func registrarErroresDeReserva(mapeador *ctrlAcceso.MapeadorDeErrores) {
	mapeador.Registrar(func(err error) bool {
		var validacion modelo.ErrValidacion
		return errors.As(err, &validacion)
	}, func(err error) vistaAcceso.RespuestaError {
		var validacion modelo.ErrValidacion
		_ = errors.As(err, &validacion)
		return vistaAcceso.RespuestaError{
			Estado: http.StatusUnprocessableEntity, Codigo: "validacion",
			Mensaje: "Revisa los datos de la reserva.", Campos: validacion.Campos,
		}
	})
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAnticipacion) },
		respuestaConCampo(http.StatusUnprocessableEntity, "anticipacion_insuficiente", "slot_id"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrHorizonte) },
		respuestaConCampo(http.StatusUnprocessableEntity, "horizonte_excedido", "slot_id"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSlotInexistente) },
		respuestaConEstado(http.StatusNotFound, "slot_inexistente"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSlotVencido) },
		respuestaConEstado(http.StatusConflict, "slot_vencido"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSlotOcupado) },
		respuestaConCampo(http.StatusConflict, "slot_ocupado", "slot_id"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrReservaNoEncontrada) },
		respuestaConEstado(http.StatusNotFound, "reserva_inexistente"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSolapamiento) },
		respuestaConEstado(http.StatusConflict, "solapamiento"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrCuotaEscolar) },
		respuestaConEstado(http.StatusConflict, "cuota_escolar"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrCuotaCursoSemana) },
		respuestaConEstado(http.StatusConflict, "cuota_curso_semana"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrIdempotenciaConflicto) },
		respuestaConEstado(http.StatusConflict, "idempotencia_conflicto"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAdjuntoNoEncontrado) },
		respuestaConEstado(http.StatusNotFound, "adjunto_inexistente"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAdjuntosLlenos) },
		respuestaConEstado(http.StatusConflict, "adjuntos_llenos"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAdjuntoEnCuarentena) },
		respuestaConEstado(http.StatusConflict, "adjunto_en_cuarentena"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAdjuntoRechazado) },
		respuestaConEstado(http.StatusConflict, "adjunto_rechazado"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAdjuntoInvalido) },
		func(error) vistaAcceso.RespuestaError {
			return vistaAcceso.RespuestaError{
				Estado: http.StatusUnprocessableEntity, Codigo: "adjunto_invalido",
				Mensaje: "El archivo no cumple el formato permitido (PDF, PNG, JPG, TXT o DOCX de hasta 10 MB).",
			}
		},
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrAlmacen) },
		func(error) vistaAcceso.RespuestaError {
			return vistaAcceso.RespuestaError{
				Estado: http.StatusInternalServerError, Codigo: "almacen",
				Mensaje: "No se pudo guardar el adjunto. Intenta de nuevo.",
			}
		},
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrSinAutorizacion) },
		respuestaConEstado(http.StatusForbidden, "sin_autorizacion"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, modelo.ErrNoAutorizado) },
		func(error) vistaAcceso.RespuestaError {
			return vistaAcceso.RespuestaError{
				Estado: http.StatusForbidden, Codigo: "no_autorizado",
				Mensaje: "No puedes acceder a esta reserva.",
			}
		},
	)
}

func respuestaConEstado(estado int, codigo string) func(error) vistaAcceso.RespuestaError {
	return func(err error) vistaAcceso.RespuestaError {
		return vistaAcceso.RespuestaError{Estado: estado, Codigo: codigo, Mensaje: err.Error()}
	}
}

func respuestaConCampo(estado int, codigo, campo string) func(error) vistaAcceso.RespuestaError {
	return func(err error) vistaAcceso.RespuestaError {
		return vistaAcceso.RespuestaError{
			Estado: estado, Codigo: codigo, Mensaje: err.Error(),
			Campos: map[string]string{campo: err.Error()},
		}
	}
}
