package acceso

import "time"

type EstadoAutorizacion string

const (
	AutorizacionOtorgada EstadoAutorizacion = "autorizado"
	AutorizacionRevocada EstadoAutorizacion = "no_autorizado"
)

// Autorizacion registra cada cambio de permiso que un apoderado hace sobre su
// escolar. El estado vigente es siempre el último registro, de modo que el
// historial conserva quién cambió qué y cuándo.
type Autorizacion struct {
	ID              string
	VinculacionID   string
	ApoderadoID     string
	EscolarID       string
	Estado          EstadoAutorizacion
	CreadaEn        time.Time
	ApoderadoNombre string
	ApoderadoEmail  string
	EscolarNombre   string
	EscolarEmail    string
}

func (a Autorizacion) Vigente() bool {
	return a.Estado == AutorizacionOtorgada
}
