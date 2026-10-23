package acceso

import (
	"context"
	"time"
)

type Repositorio interface {
	CrearUsuario(ctx context.Context, u *Usuario) (*Usuario, error)
	UsuarioPorEmail(ctx context.Context, email string) (*Usuario, error)
	UsuarioPorID(ctx context.Context, id string) (*Usuario, error)
	MarcarCorreoConfirmado(ctx context.Context, id string) error
	ActualizarClave(ctx context.Context, id, hash string) error
	RegistrarIngreso(ctx context.Context, id string, t time.Time) error

	GuardarSesion(ctx context.Context, s Sesion) error
	SesionPorHash(ctx context.Context, hash []byte) (*Sesion, error)
	EliminarSesion(ctx context.Context, hash []byte) error
	EliminarSesionesDeUsuario(ctx context.Context, id string) error

	GuardarTokenRecuperacion(ctx context.Context, t TokenRecuperacion) error

	UsuarioDeTokenRecuperacion(ctx context.Context, hash []byte, ahora time.Time) (string, error)

	ConsumirTokenRecuperacion(ctx context.Context, hash []byte, ahora time.Time) (string, error)

	GuardarTokenConfirmacion(ctx context.Context, t TokenConfirmacion) error

	ConsumirTokenConfirmacion(ctx context.Context, hash []byte, ahora time.Time) (string, error)

	EliminarUsuariosSinConfirmar(ctx context.Context, ahora time.Time) (int64, error)

	GuardarSolicitudVinculacion(ctx context.Context, v Vinculacion) (*Vinculacion, error)
	SolicitudPendienteDeEscolar(ctx context.Context, escolarID string, ahora time.Time) (*Vinculacion, error)
	SolicitudVinculacionPorToken(ctx context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error)
	VinculacionAceptadaDeEscolar(ctx context.Context, escolarID string) (*Vinculacion, error)
	ConsumirSolicitudVinculacion(ctx context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error)
	VinculacionesDeApoderado(ctx context.Context, apoderadoID string, ahora time.Time) ([]Vinculacion, error)

	AutorizacionVigenteDeEscolar(ctx context.Context, escolarID string) (*Autorizacion, error)
	AutorizacionesDeVinculacion(ctx context.Context, vinculacionID string) ([]Autorizacion, error)
	CambiarAutorizacion(ctx context.Context, vinculacionID, escolarID, apoderadoID string, estado EstadoAutorizacion, ahora time.Time) (*Autorizacion, error)
}
