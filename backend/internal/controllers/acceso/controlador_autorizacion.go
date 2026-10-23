package acceso

import (
	"net/http"

	modelo "uniapp/internal/models/acceso"
	vista "uniapp/internal/views/acceso"
)

func (c *Controlador) cambiarAutorizacion(w http.ResponseWriter, r *http.Request) {
	var entrada struct {
		Autorizado *bool `json:"autorizado"`
	}
	if !c.decodificarCuerpo(w, r, &entrada) {
		return
	}
	if entrada.Autorizado == nil {
		c.responderError(w, modelo.ErrValidacion{
			Campos: map[string]string{"autorizado": "indica si el escolar puede reservar"},
		})
		return
	}

	autorizacion, err := c.servicio.CambiarAutorizacion(
		r.Context(),
		UsuarioDe(r.Context()),
		r.PathValue("escolarID"),
		*entrada.Autorizado,
	)
	if err != nil {
		c.responderError(w, err)
		return
	}

	mensaje := "Autorización revocada. El escolar no podrá crear nuevas reservas."
	if autorizacion.Vigente() {
		mensaje = "Autorización otorgada. El escolar puede crear nuevas reservas."
	}
	vista.EscribirJSON(w, http.StatusOK, map[string]any{
		"mensaje":      mensaje,
		"autorizacion": vista.PresentarAutorizacion(autorizacion),
	})
}
