package acceso

import (
	"encoding/json"
	"net/http"
	"time"

	modelo "uniapp/internal/models/acceso"
)

type UsuarioJSON struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	NombreUso        string `json:"nombre_uso"`
	Rol              string `json:"rol"`
	CorreoConfirmado bool   `json:"correo_confirmado"`
}

func PresentarUsuario(usuario *modelo.Usuario) UsuarioJSON {
	return UsuarioJSON{
		ID: usuario.ID, Email: usuario.Email, NombreUso: usuario.NombreUso,
		Rol: string(usuario.Rol), CorreoConfirmado: usuario.EmailConfirmado,
	}
}

type PanelCoordinacionJSON struct {
	Mensaje string      `json:"mensaje"`
	Usuario UsuarioJSON `json:"usuario"`
}

func PresentarPanelCoordinacion(usuario *modelo.Usuario) PanelCoordinacionJSON {
	return PanelCoordinacionJSON{
		Mensaje: "Panel de coordinación",
		Usuario: PresentarUsuario(usuario),
	}
}

type ErrorJSON struct {
	Codigo  string            `json:"codigo"`
	Mensaje string            `json:"mensaje"`
	Campos  map[string]string `json:"campos,omitempty"`
}

type RespuestaError struct {
	Estado  int
	Codigo  string
	Mensaje string
	Campos  map[string]string
}

func EscribirJSON(w http.ResponseWriter, estado int, valor any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(estado)
	if valor != nil {
		_ = json.NewEncoder(w).Encode(valor)
	}
}

func EscribirError(w http.ResponseWriter, respuesta RespuestaError) {
	EscribirJSON(w, respuesta.Estado, ErrorJSON{Codigo: respuesta.Codigo, Mensaje: respuesta.Mensaje, Campos: respuesta.Campos})
}

type VinculacionUsuarioJSON struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	NombreUso string `json:"nombre_uso"`
	Rol       string `json:"rol"`
}

type VinculacionJSON struct {
	ID           string                 `json:"id"`
	Estado       string                 `json:"estado"`
	Apoderado    VinculacionUsuarioJSON `json:"apoderado"`
	Escolar      VinculacionUsuarioJSON `json:"escolar"`
	Autorizacion *AutorizacionJSON      `json:"autorizacion,omitempty"`
	CreadaEn     time.Time              `json:"creada_en"`
	AceptadaEn   *time.Time             `json:"aceptada_en,omitempty"`
	ExpiraEn     time.Time              `json:"expira_en"`
}

type AutorizacionJSON struct {
	ID         string    `json:"id"`
	Estado     string    `json:"estado"`
	Autorizado bool      `json:"autorizado"`
	CreadaEn   time.Time `json:"creada_en"`
}

func PresentarAutorizacion(autorizacion *modelo.Autorizacion) *AutorizacionJSON {
	if autorizacion == nil {
		return nil
	}
	return &AutorizacionJSON{
		ID:         autorizacion.ID,
		Estado:     string(autorizacion.Estado),
		Autorizado: autorizacion.Vigente(),
		CreadaEn:   autorizacion.CreadaEn,
	}
}

func PresentarAutorizaciones(autorizaciones []modelo.Autorizacion) []AutorizacionJSON {
	presentadas := make([]AutorizacionJSON, 0, len(autorizaciones))
	for i := range autorizaciones {
		presentadas = append(presentadas, *PresentarAutorizacion(&autorizaciones[i]))
	}
	return presentadas
}

func PresentarVinculacion(vinculacion *modelo.Vinculacion) VinculacionJSON {
	return VinculacionJSON{
		ID:     vinculacion.ID,
		Estado: string(vinculacion.Estado),
		Apoderado: VinculacionUsuarioJSON{
			ID: vinculacion.ApoderadoID, Email: vinculacion.ApoderadoEmail,
			NombreUso: vinculacion.ApoderadoNombre, Rol: string(modelo.RolApoderado),
		},
		Escolar: VinculacionUsuarioJSON{
			ID: vinculacion.EscolarID, Email: vinculacion.EscolarEmail,
			NombreUso: vinculacion.EscolarNombre, Rol: string(modelo.RolEscolar),
		},
		CreadaEn:     vinculacion.CreadaEn,
		AceptadaEn:   vinculacion.AceptadaEn,
		ExpiraEn:     vinculacion.ExpiraEn,
		Autorizacion: PresentarAutorizacion(vinculacion.Autorizacion),
	}
}

func PresentarVinculaciones(vinculaciones []modelo.Vinculacion) []VinculacionJSON {
	presentadas := make([]VinculacionJSON, 0, len(vinculaciones))
	for i := range vinculaciones {
		presentadas = append(presentadas, PresentarVinculacion(&vinculaciones[i]))
	}
	return presentadas
}
