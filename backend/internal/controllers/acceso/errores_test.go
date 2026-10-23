package acceso

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	modelo "uniapp/internal/models/acceso"
)

func TestMapeadorDeErroresSeExtiendeSinCambiarElRespondedor(t *testing.T) {
	mapeador := NuevoMapeadorDeErrores()
	errPropio := errors.New("error propio")
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, errPropio) },
		respuestaConEstado(http.StatusTeapot, "propio"),
	)

	w := httptest.NewRecorder()
	mapeador.Escribir(w, errPropio)
	if w.Code != http.StatusTeapot || !strings.Contains(w.Body.String(), `"codigo":"propio"`) {
		t.Fatalf("regla propia: %d %s", w.Code, w.Body)
	}

	w = httptest.NewRecorder()
	mapeador.Escribir(w, errors.New("desconocido"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"codigo":"error_interno"`) {
		t.Fatalf("respuesta por defecto: %d %s", w.Code, w.Body)
	}
}

func TestMapeadorDeErroresRespetaElOrdenDeLaCadena(t *testing.T) {
	mapeador := NuevoMapeadorDeErrores()
	errPropio := errors.New("error propio")
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, errPropio) },
		respuestaConEstado(http.StatusTeapot, "primero"),
	)
	mapeador.Registrar(
		func(err error) bool { return errors.Is(err, errPropio) },
		respuestaConEstado(http.StatusInternalServerError, "segundo"),
	)

	w := httptest.NewRecorder()
	mapeador.Escribir(w, errPropio)
	if w.Code != http.StatusTeapot || !strings.Contains(w.Body.String(), `"codigo":"primero"`) {
		t.Fatalf("debe ganar la primera regla: %d %s", w.Code, w.Body)
	}
}

func TestMapeadorDeErroresDeCuentaMantieneValidacion(t *testing.T) {
	mapeador := NuevoMapeadorDeErrores()
	registrarErroresDeCuenta(mapeador)

	w := httptest.NewRecorder()
	mapeador.Escribir(w, modelo.ErrValidacion{Campos: map[string]string{"email": "obligatorio"}})
	if w.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(w.Body.String(), `"codigo":"validacion"`) ||
		!strings.Contains(w.Body.String(), `"email":"obligatorio"`) {
		t.Fatalf("validación: %d %s", w.Code, w.Body)
	}
}
