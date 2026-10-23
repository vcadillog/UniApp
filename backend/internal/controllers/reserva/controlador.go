package reserva

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	ctrlAcceso "uniapp/internal/controllers/acceso"
	modeloAcceso "uniapp/internal/models/acceso"
	modelo "uniapp/internal/models/reserva"
	vistaAcceso "uniapp/internal/views/acceso"
	vista "uniapp/internal/views/reserva"
)

// limiteCuerpoAdjunto deja margen para el formulario multipart sobre el tope del archivo.
const limiteCuerpoAdjunto = modelo.AdjuntoMaximoBytes + (1 << 20)

// Controlador expone la reserva y sus adjuntos. Recibe el decorador de sesión con su tipo
// sin nombre para poder usar el mismo Autenticar del flujo A sin acoplarse a su paquete.
type Controlador struct {
	servicio   *modelo.Servicio
	autenticar func(http.Handler) http.Handler
	errores    *ctrlAcceso.MapeadorDeErrores
}

func Nuevo(servicio *modelo.Servicio, autenticar func(http.Handler) http.Handler) *Controlador {
	errores := ctrlAcceso.NuevoMapeadorDeErrores()
	registrarErroresDeReserva(errores)
	return &Controlador{servicio: servicio, autenticar: autenticar, errores: errores}
}

func (c *Controlador) Rutas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/reserva/salud", c.salud)
	mux.Handle("GET /api/reserva/slots", c.autenticar(http.HandlerFunc(c.slots)))
	mux.Handle("GET /api/reserva/{id}", c.autenticar(http.HandlerFunc(c.consultar)))
	mux.Handle("POST /api/reserva", c.autenticar(http.HandlerFunc(c.crear)))
	mux.Handle("GET /api/reserva/{id}/adjuntos", c.autenticar(http.HandlerFunc(c.listarAdjuntos)))
	mux.Handle("POST /api/reserva/{id}/adjuntos", c.autenticar(http.HandlerFunc(c.subirAdjunto)))
	mux.Handle("GET /api/reserva/{id}/adjuntos/{adjuntoId}", c.autenticar(http.HandlerFunc(c.descargarAdjunto)))
	return mux
}

func (c *Controlador) salud(w http.ResponseWriter, _ *http.Request) {
	vistaAcceso.EscribirJSON(w, http.StatusOK, map[string]string{"flujo": "reserva", "estado": "implementado"})
}

func (c *Controlador) slots(w http.ResponseWriter, r *http.Request) {
	if ctrlAcceso.UsuarioDe(r.Context()) == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	disposiciones, err := c.servicio.Slots(r.Context())
	if err != nil {
		c.responderError(w, err)
		return
	}
	vistaAcceso.EscribirJSON(w, http.StatusOK, map[string]any{"slots": vista.PresentarSlots(disposiciones)})
}

func (c *Controlador) crear(w http.ResponseWriter, r *http.Request) {
	usuario := ctrlAcceso.UsuarioDe(r.Context())
	if usuario == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	// Solo el escolar reserva; el apoderado gestiona autorizaciones (I06) y el tutor
	// publica disponibilidad (I03).
	if usuario.Rol != modeloAcceso.RolEscolar {
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusForbidden, Codigo: "solo_escolar",
			Mensaje: "Solo una cuenta de escolar puede crear reservas.",
		})
		return
	}
	var entrada struct {
		SlotID   string `json:"slot_id"`
		Tema     string `json:"tema"`
		Objetivo string `json:"objetivo"`
		Grado    string `json:"grado"`
	}
	if !decodificarCuerpo(w, r, &entrada) {
		return
	}
	datos := modelo.DatosReserva{
		SlotID: entrada.SlotID, Tema: entrada.Tema, Objetivo: entrada.Objetivo, Grado: entrada.Grado,
		ClaveIdempotencia: r.Header.Get("Idempotency-Key"),
	}
	reserva, creada, err := c.servicio.Reservar(r.Context(), usuario.ID, datos)
	if err != nil {
		c.responderError(w, err)
		return
	}
	if creada {
		w.Header().Set("Idempotent-Replayed", "false")
	} else {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	estado := http.StatusCreated
	if !creada {
		estado = http.StatusOK
	}
	vistaAcceso.EscribirJSON(w, estado, vista.PresentarReserva(reserva))
}

func (c *Controlador) consultar(w http.ResponseWriter, r *http.Request) {
	usuario := ctrlAcceso.UsuarioDe(r.Context())
	if usuario == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	reserva, err := c.servicio.ReservaDe(r.Context(), r.PathValue("id"), usuario.ID)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vistaAcceso.EscribirJSON(w, http.StatusOK, vista.PresentarReserva(reserva))
}

func (c *Controlador) listarAdjuntos(w http.ResponseWriter, r *http.Request) {
	usuario := ctrlAcceso.UsuarioDe(r.Context())
	if usuario == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	adjuntos, err := c.servicio.AdjuntosDe(r.Context(), usuario.ID, usuario.Rol, r.PathValue("id"))
	if err != nil {
		c.responderError(w, err)
		return
	}
	vistaAcceso.EscribirJSON(w, http.StatusOK, map[string]any{"adjuntos": vista.PresentarAdjuntos(adjuntos)})
}

func (c *Controlador) subirAdjunto(w http.ResponseWriter, r *http.Request) {
	usuario := ctrlAcceso.UsuarioDe(r.Context())
	if usuario == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	if usuario.Rol != modeloAcceso.RolEscolar {
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusForbidden, Codigo: "solo_escolar",
			Mensaje: "Solo el escolar dueño de la reserva sube adjuntos.",
		})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limiteCuerpoAdjunto)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var demasiadoGrande *http.MaxBytesError
		if errors.As(err, &demasiadoGrande) {
			vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
				Estado: http.StatusRequestEntityTooLarge, Codigo: "adjunto_muy_grande",
				Mensaje: "Cada adjunto debe pesar como máximo 10 MB.",
			})
			return
		}
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusBadRequest, Codigo: "solicitud_invalida",
			Mensaje: "Envía el archivo en el campo 'archivo' como multipart/form-data.",
		})
		return
	}
	archivo, cabecera, err := r.FormFile("archivo")
	if err != nil {
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusBadRequest, Codigo: "solicitud_invalida",
			Mensaje: "Falta el archivo en el campo 'archivo'.",
		})
		return
	}
	defer func() { _ = archivo.Close() }()
	contenido, err := io.ReadAll(io.LimitReader(archivo, modelo.AdjuntoMaximoBytes+1))
	if err != nil {
		c.responderError(w, err)
		return
	}
	if int64(len(contenido)) > modelo.AdjuntoMaximoBytes {
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusRequestEntityTooLarge, Codigo: "adjunto_muy_grande",
			Mensaje: "Cada adjunto debe pesar como máximo 10 MB.",
		})
		return
	}
	adjunto, err := c.servicio.Adjuntar(r.Context(), usuario.ID, r.PathValue("id"), cabecera.Filename, contenido)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vistaAcceso.EscribirJSON(w, http.StatusCreated, vista.PresentarAdjunto(adjunto))
}

func (c *Controlador) descargarAdjunto(w http.ResponseWriter, r *http.Request) {
	usuario := ctrlAcceso.UsuarioDe(r.Context())
	if usuario == nil {
		c.responderError(w, modelo.ErrNoAutorizado)
		return
	}
	adjunto, contenido, err := c.servicio.AbrirAdjunto(r.Context(), usuario.ID, usuario.Rol, r.PathValue("id"), r.PathValue("adjuntoId"))
	if err != nil {
		c.responderError(w, err)
		return
	}
	disposicion := mime.FormatMediaType("attachment", map[string]string{"filename": adjunto.Nombre})
	w.Header().Set("Content-Type", adjunto.Tipo)
	w.Header().Set("Content-Disposition", disposicion)
	w.Header().Set("Content-Length", strconv.Itoa(len(contenido)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contenido)
}

func decodificarCuerpo(w http.ResponseWriter, r *http.Request, destino any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decodificador := json.NewDecoder(r.Body)
	decodificador.DisallowUnknownFields()
	if err := decodificador.Decode(destino); err != nil {
		vistaAcceso.EscribirError(w, vistaAcceso.RespuestaError{
			Estado: http.StatusBadRequest, Codigo: "solicitud_invalida",
			Mensaje: "El cuerpo de la solicitud no es válido.",
		})
		return false
	}
	return true
}

func (c *Controlador) responderError(w http.ResponseWriter, err error) {
	c.errores.Escribir(w, err)
}
