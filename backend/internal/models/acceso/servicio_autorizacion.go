package acceso

import (
	"context"
	"errors"
	"strings"
)

// CambiarAutorizacion permite al apoderado revocar u otorgar el permiso para
// crear nuevas reservas. No modifica reservas ya confirmadas: la comprobación
// ocurre únicamente al intentar reservar.
func (s *Servicio) CambiarAutorizacion(ctx context.Context, apoderado *Usuario, escolarID string, autorizado bool) (*Autorizacion, error) {
	vinculacion, err := s.vinculacionDelApoderado(ctx, apoderado, escolarID)
	if err != nil {
		return nil, err
	}

	estado := AutorizacionRevocada
	if autorizado {
		estado = AutorizacionOtorgada
	}
	return s.repositorio.CambiarAutorizacion(ctx, vinculacion.ID, vinculacion.EscolarID, apoderado.ID, estado, s.ahora())
}

// HistorialAutorizaciones devuelve cada cambio de permiso del vínculo, del más
// reciente al más antiguo, para que el apoderado pueda auditar cuándo otorgó o
// retiró la capacidad de pedir nuevas tutorías.
func (s *Servicio) HistorialAutorizaciones(ctx context.Context, apoderado *Usuario, escolarID string) ([]Autorizacion, error) {
	vinculacion, err := s.vinculacionDelApoderado(ctx, apoderado, escolarID)
	if err != nil {
		return nil, err
	}
	return s.repositorio.AutorizacionesDeVinculacion(ctx, vinculacion.ID)
}

// vinculacionDelApoderado comprueba rol y propiedad del vínculo. El apoderado
// solo puede consultar o cambiar escolares que haya vinculado y aceptado.
func (s *Servicio) vinculacionDelApoderado(ctx context.Context, apoderado *Usuario, escolarID string) (*Vinculacion, error) {
	if apoderado == nil || apoderado.Rol != RolApoderado || !apoderado.Activo {
		return nil, ErrNoEsApoderado
	}
	escolarID = strings.TrimSpace(escolarID)
	if escolarID == "" {
		return nil, ErrValidacion{Campos: map[string]string{"escolar_id": "el escolar es obligatorio"}}
	}

	vinculacion, err := s.repositorio.VinculacionAceptadaDeEscolar(ctx, escolarID)
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrSinApoderado
	}
	if err != nil {
		return nil, err
	}
	if vinculacion.ApoderadoID != apoderado.ID {
		return nil, ErrVinculacionAjena
	}
	return vinculacion, nil
}

// AutorizacionVigente responde si el escolar puede crear nuevas reservas. La
// ausencia de un vínculo o de una autorización equivale a no autorizado.
func (s *Servicio) AutorizacionVigente(ctx context.Context, escolarID string) (bool, error) {
	autorizacion, err := s.repositorio.AutorizacionVigenteDeEscolar(ctx, strings.TrimSpace(escolarID))
	if errors.Is(err, ErrNoEncontrado) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return autorizacion.Vigente(), nil
}
