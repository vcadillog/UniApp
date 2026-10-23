package acceso

import (
	"errors"
	"net/http"

	vista "uniapp/internal/views/acceso"
)

// manejadorDeError implementa un eslabón de la cadena de responsabilidad.
// Cada regla decide si maneja el error; si no, lo delega al siguiente.
type manejadorDeError interface {
	manejar(error) (vista.RespuestaError, bool)
	establecerSiguiente(manejadorDeError)
}

type reglaDeError struct {
	coincide  func(error) bool
	respuesta func(error) vista.RespuestaError
	siguiente manejadorDeError
}

func (r *reglaDeError) manejar(err error) (vista.RespuestaError, bool) {
	if r.coincide(err) {
		return r.respuesta(err), true
	}
	if r.siguiente != nil {
		return r.siguiente.manejar(err)
	}
	return vista.RespuestaError{}, false
}

func (r *reglaDeError) establecerSiguiente(siguiente manejadorDeError) {
	r.siguiente = siguiente
}

type MapeadorDeErrores struct {
	primero manejadorDeError
	ultimo  *reglaDeError
}

func NuevoMapeadorDeErrores() *MapeadorDeErrores {
	return &MapeadorDeErrores{}
}

func (m *MapeadorDeErrores) Registrar(coincide func(error) bool, respuesta func(error) vista.RespuestaError) {
	regla := &reglaDeError{coincide: coincide, respuesta: respuesta}
	if m.primero == nil {
		m.primero = regla
	} else {
		m.ultimo.establecerSiguiente(regla)
	}
	m.ultimo = regla
}

func (m *MapeadorDeErrores) Escribir(w http.ResponseWriter, err error) {
	if m.primero != nil {
		if respuesta, manejado := m.primero.manejar(err); manejado {
			vista.EscribirError(w, respuesta)
			return
		}
	}
	vista.EscribirError(w, vista.RespuestaError{
		Estado:  http.StatusInternalServerError,
		Codigo:  "error_interno",
		Mensaje: "Ocurrió un error inesperado.",
	})
}

func esAlguno(err error, objetivos ...error) bool {
	for _, objetivo := range objetivos {
		if errors.Is(err, objetivo) {
			return true
		}
	}
	return false
}

func respuestaConEstado(estado int, codigo string) func(error) vista.RespuestaError {
	return func(err error) vista.RespuestaError {
		return vista.RespuestaError{Estado: estado, Codigo: codigo, Mensaje: err.Error()}
	}
}

func respuestaConCampos(estado int, codigo, campo string) func(error) vista.RespuestaError {
	return func(err error) vista.RespuestaError {
		return vista.RespuestaError{
			Estado: estado, Codigo: codigo, Mensaje: err.Error(),
			Campos: map[string]string{campo: err.Error()},
		}
	}
}
