package acceso

import (
	"encoding/json"
	"net/http"
	"time"

	modelo "uniapp/internal/models/acceso"
	vista "uniapp/internal/views/acceso"
)

const (
	nombreCookie = "uniapp_sesion"
	rutaCookie   = "/"
)

type Controlador struct {
	servicio     *modelo.Servicio
	cookieSegura bool
	errores      *MapeadorDeErrores
}

func Nuevo(servicio *modelo.Servicio, cookieSegura bool) *Controlador {
	controlador := &Controlador{
		servicio:     servicio,
		cookieSegura: cookieSegura,
		errores:      NuevoMapeadorDeErrores(),
	}
	registrarErroresDeCuenta(controlador.errores)
	registrarErroresDeVinculacion(controlador.errores)
	return controlador
}

func (c *Controlador) Rutas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/registro", c.registro)
	mux.HandleFunc("POST /api/auth/ingreso", c.ingreso)
	mux.HandleFunc("POST /api/auth/recuperar", c.recuperar)
	mux.HandleFunc("POST /api/auth/restablecer", c.restablecer)
	mux.HandleFunc("POST /api/auth/confirmar", c.confirmar)
	mux.HandleFunc("POST /api/auth/reenviar-confirmacion", c.reenviarConfirmacion)
	mux.Handle("POST /api/auth/salida", Encadenar(http.HandlerFunc(c.salida), c.Autenticar()))
	mux.Handle("GET /api/auth/yo", Encadenar(http.HandlerFunc(c.yo), c.Autenticar()))
	mux.Handle("GET /api/coordinacion/panel", Encadenar(http.HandlerFunc(c.panelCoordinacion), c.Autenticar(), RequierePermiso(modelo.PermisoCoordinacionAcceder)))
	mux.Handle("POST /api/apoderado/vinculaciones", Encadenar(http.HandlerFunc(c.solicitarVinculacion), c.Autenticar()))
	mux.Handle("GET /api/apoderado/vinculaciones", Encadenar(http.HandlerFunc(c.listarVinculaciones), c.Autenticar()))
	mux.Handle("POST /api/apoderado/escolares/{escolarID}/autorizacion", Encadenar(http.HandlerFunc(c.cambiarAutorizacion), c.Autenticar(), RequierePermiso(modelo.PermisoAutorizacionOtorgar)))
	mux.Handle("GET /api/apoderado/escolares/{escolarID}/autorizaciones", Encadenar(http.HandlerFunc(c.historialAutorizaciones), c.Autenticar(), RequierePermiso(modelo.PermisoAutorizacionOtorgar)))
	mux.Handle("POST /api/escolar/vinculaciones/consultar", Encadenar(http.HandlerFunc(c.consultarVinculacion), c.Autenticar()))
	mux.Handle("POST /api/escolar/vinculaciones/aceptar", Encadenar(http.HandlerFunc(c.aceptarVinculacion), c.Autenticar()))
	mux.Handle("GET /api/escolar/apoderado", Encadenar(http.HandlerFunc(c.miApoderado), c.Autenticar()))
	mux.HandleFunc("GET /api/salud", c.salud)
	return mux
}

func (c *Controlador) cookieBase() *http.Cookie {
	return &http.Cookie{
		Name:     nombreCookie,
		Path:     rutaCookie,
		HttpOnly: true,
		Secure:   c.cookieSegura,
		SameSite: http.SameSiteStrictMode,
	}
}

func (c *Controlador) cookieDeSesion(token string) *http.Cookie {
	cookie := c.cookieBase()
	cookie.Value = token
	cookie.Expires = time.Now().Add(modelo.DuracionSesion)
	return cookie
}

func (c *Controlador) cookieExpirada() *http.Cookie {
	cookie := c.cookieBase()
	cookie.MaxAge = -1
	return cookie
}

func (c *Controlador) salud(w http.ResponseWriter, _ *http.Request) {
	vista.EscribirJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
}

func (c *Controlador) decodificarCuerpo(w http.ResponseWriter, r *http.Request, destino any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decodificador := json.NewDecoder(r.Body)
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(destino); err != nil {
		vista.EscribirError(w, vista.RespuestaError{
			Estado:  http.StatusBadRequest,
			Codigo:  "solicitud_invalida",
			Mensaje: "El cuerpo de la solicitud no es válido.",
		})
		return false
	}
	return true
}

func (c *Controlador) responderError(w http.ResponseWriter, err error) {
	c.errores.Escribir(w, err)
}
