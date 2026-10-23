package acceso

import (
	"net/http"

	vista "uniapp/internal/views/acceso"
)

func (c *Controlador) solicitarVinculacion(w http.ResponseWriter, r *http.Request) {
	var entrada struct {
		EmailEscolar string `json:"email_escolar"`
	}
	if !c.decodificarCuerpo(w, r, &entrada) {
		return
	}
	vinculacion, err := c.servicio.SolicitarVinculacion(r.Context(), UsuarioDe(r.Context()), entrada.EmailEscolar)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vista.EscribirJSON(w, http.StatusAccepted, map[string]any{
		"mensaje":     "Solicitud enviada. El escolar recibirá un enlace para aceptarla.",
		"vinculacion": vista.PresentarVinculacion(vinculacion),
	})
}

func (c *Controlador) listarVinculaciones(w http.ResponseWriter, r *http.Request) {
	vinculaciones, err := c.servicio.VinculacionesDeApoderado(r.Context(), UsuarioDe(r.Context()))
	if err != nil {
		c.responderError(w, err)
		return
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{
		"vinculaciones": vista.PresentarVinculaciones(vinculaciones),
	})
}

// historialAutorizaciones expone, para un escolar del apoderado autenticado,
// cada vez que se otorgó o retiró el permiso de pedir nuevas tutorías.
func (c *Controlador) historialAutorizaciones(w http.ResponseWriter, r *http.Request) {
	autorizaciones, err := c.servicio.HistorialAutorizaciones(
		r.Context(), UsuarioDe(r.Context()), r.PathValue("escolarID"),
	)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{
		"autorizaciones": vista.PresentarAutorizaciones(autorizaciones),
	})
}

func (c *Controlador) consultarVinculacion(w http.ResponseWriter, r *http.Request) {
	var entrada struct {
		Token string `json:"token"`
	}
	if !c.decodificarCuerpo(w, r, &entrada) {
		return
	}
	vinculacion, err := c.servicio.ConsultarVinculacion(r.Context(), UsuarioDe(r.Context()), entrada.Token)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{
		"vinculacion": vista.PresentarVinculacion(vinculacion),
	})
}

func (c *Controlador) aceptarVinculacion(w http.ResponseWriter, r *http.Request) {
	var entrada struct {
		Token string `json:"token"`
	}
	if !c.decodificarCuerpo(w, r, &entrada) {
		return
	}
	vinculacion, err := c.servicio.AceptarVinculacion(r.Context(), UsuarioDe(r.Context()), entrada.Token)
	if err != nil {
		c.responderError(w, err)
		return
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{
		"mensaje":     "Vinculación aceptada. Tu apoderado ya está registrado.",
		"vinculacion": vista.PresentarVinculacion(vinculacion),
	})
}

func (c *Controlador) miApoderado(w http.ResponseWriter, r *http.Request) {
	vinculacion, err := c.servicio.VinculacionDeEscolar(r.Context(), UsuarioDe(r.Context()))
	if err != nil {
		c.responderError(w, err)
		return
	}
	var presentada any
	if vinculacion != nil {
		presentada = vista.PresentarVinculacion(vinculacion)
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{"vinculacion": presentada})
}
