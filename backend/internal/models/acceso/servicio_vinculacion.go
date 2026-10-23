package acceso

import (
	"context"
	"errors"
	"time"
)

const duracionVinculacion = 24 * time.Hour

// SolicitarVinculacion crea un enlace de un solo uso para que el escolar
// confirme a su apoderado. El apoderado puede tener varios escolares, pero un
// escolar solo puede tener un apoderado aceptado.
func (s *Servicio) SolicitarVinculacion(ctx context.Context, apoderado *Usuario, emailEscolar string) (*Vinculacion, error) {
	if apoderado == nil || apoderado.Rol != RolApoderado || !apoderado.Activo {
		return nil, ErrNoEsApoderado
	}
	emailEscolar = normalizarEmail(emailEscolar)
	if emailEscolar == "" {
		return nil, ErrValidacion{Campos: map[string]string{"email_escolar": "el correo es obligatorio"}}
	}

	escolar, err := s.repositorio.UsuarioPorEmail(ctx, emailEscolar)
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrEscolarNoEncontrado
	}
	if err != nil {
		return nil, err
	}
	if escolar.Rol != RolEscolar || !escolar.Activo || !escolar.EmailConfirmado {
		return nil, ErrEscolarNoValido
	}

	if _, err := s.repositorio.VinculacionAceptadaDeEscolar(ctx, escolar.ID); err == nil {
		return nil, ErrYaVinculado
	} else if !errors.Is(err, ErrNoEncontrado) {
		return nil, err
	}

	if pendiente, err := s.repositorio.SolicitudPendienteDeEscolar(ctx, escolar.ID, s.ahora()); err == nil {
		if pendiente.ApoderadoID == apoderado.ID {
			return nil, ErrSolicitudPendiente
		}
		return nil, ErrSolicitudPendiente
	} else if !errors.Is(err, ErrNoEncontrado) {
		return nil, err
	}

	token, tokenHash, err := nuevoToken()
	if err != nil {
		return nil, err
	}
	momento := s.ahora()
	vinculacion := Vinculacion{
		ApoderadoID:     apoderado.ID,
		EscolarID:       escolar.ID,
		Estado:          EstadoVinculacionPendiente,
		TokenHash:       tokenHash,
		ExpiraEn:        momento.Add(duracionVinculacion),
		CreadaEn:        momento,
		ApoderadoNombre: apoderado.NombreUso,
		ApoderadoEmail:  apoderado.Email,
		EscolarNombre:   escolar.NombreUso,
		EscolarEmail:    escolar.Email,
	}
	creada, err := s.repositorio.GuardarSolicitudVinculacion(ctx, vinculacion)
	if err != nil {
		return nil, err
	}

	s.bus.Notificar(Event{Nombre: VinculacionSolicitada, Payload: map[string]any{
		"usuario_id":       apoderado.ID,
		"rol":              string(apoderado.Rol),
		"apoderado_id":     apoderado.ID,
		"escolar_id":       escolar.ID,
		"apoderado_nombre": apoderado.NombreUso,
		"apoderado_email":  apoderado.Email,
		"escolar_nombre":   escolar.NombreUso,
		"email":            escolar.Email,
		"token":            token,
	}})
	return creada, nil
}

func (s *Servicio) ConsultarVinculacion(ctx context.Context, escolar *Usuario, token string) (*Vinculacion, error) {
	if escolar == nil || escolar.Rol != RolEscolar || !escolar.Activo {
		return nil, ErrNoEsEscolar
	}
	if token == "" {
		return nil, ErrTokenInvalido
	}
	vinculacion, err := s.repositorio.SolicitudVinculacionPorToken(ctx, hashToken(token), escolar.ID, s.ahora())
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrTokenInvalido
	}
	return vinculacion, err
}

func (s *Servicio) AceptarVinculacion(ctx context.Context, escolar *Usuario, token string) (*Vinculacion, error) {
	if escolar == nil || escolar.Rol != RolEscolar || !escolar.Activo {
		return nil, ErrNoEsEscolar
	}
	if token == "" {
		return nil, ErrTokenInvalido
	}
	vinculacion, err := s.repositorio.ConsumirSolicitudVinculacion(ctx, hashToken(token), escolar.ID, s.ahora())
	if err != nil {
		return nil, err
	}

	s.bus.Notificar(Event{Nombre: VinculacionAceptada, Payload: map[string]any{
		"usuario_id":       escolar.ID,
		"rol":              string(escolar.Rol),
		"apoderado_id":     vinculacion.ApoderadoID,
		"escolar_id":       vinculacion.EscolarID,
		"apoderado_nombre": vinculacion.ApoderadoNombre,
		"escolar_nombre":   vinculacion.EscolarNombre,
		"apoderado_email":  vinculacion.ApoderadoEmail,
	}})
	return vinculacion, nil
}

func (s *Servicio) VinculacionesDeApoderado(ctx context.Context, apoderado *Usuario) ([]Vinculacion, error) {
	if apoderado == nil || apoderado.Rol != RolApoderado || !apoderado.Activo {
		return nil, ErrNoEsApoderado
	}
	return s.repositorio.VinculacionesDeApoderado(ctx, apoderado.ID, s.ahora())
}

func (s *Servicio) VinculacionDeEscolar(ctx context.Context, escolar *Usuario) (*Vinculacion, error) {
	if escolar == nil || escolar.Rol != RolEscolar || !escolar.Activo {
		return nil, ErrNoEsEscolar
	}
	vinculacion, err := s.repositorio.VinculacionAceptadaDeEscolar(ctx, escolar.ID)
	if errors.Is(err, ErrNoEncontrado) {
		return nil, nil
	}
	return vinculacion, err
}
