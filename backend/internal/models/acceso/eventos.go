package acceso

const (
	UsuarioRegistrado      = "usuario.registrado"
	IngresoExitoso         = "ingreso.exitoso"
	IngresoFallido         = "ingreso.fallido"
	RecuperacionSolicitada = "clave.recuperacion_solicitada"
	ClaveRestablecida      = "clave.restablecida"
	ConfirmacionSolicitada = "correo.confirmacion_solicitada"
	CorreoConfirmado       = "correo.confirmado"
	VinculacionSolicitada  = "vinculacion.solicitada"
	VinculacionAceptada    = "vinculacion.aceptada"
)

type Event struct {
	Nombre  string
	Payload map[string]any
}

type Observador interface {
	Actualizar(Event)
	ID() string
}

type Sujeto interface {
	Registrar(nombre string, observador Observador)
	Desregistrar(nombre string, observador Observador)
	Notificar(Event)
}
