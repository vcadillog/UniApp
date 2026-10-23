package acceso

type FabricaRol interface {
	CrearPerfil(dominiosTutor string) PerfilRol
	Permisos() []Permiso
}

type fabricaRolEscolar struct{}

func (fabricaRolEscolar) CrearPerfil(string) PerfilRol { return nuevoPerfilEscolar() }
func (fabricaRolEscolar) Permisos() []Permiso          { return []Permiso{PermisoReservaCrear} }

type fabricaRolApoderado struct{}

func (fabricaRolApoderado) CrearPerfil(string) PerfilRol { return nuevoPerfilApoderado() }
func (fabricaRolApoderado) Permisos() []Permiso          { return []Permiso{PermisoAutorizacionOtorgar} }

type fabricaRolTutor struct{}

func (fabricaRolTutor) CrearPerfil(dominiosTutor string) PerfilRol {
	return nuevoPerfilTutor(dominiosTutor)
}
func (fabricaRolTutor) Permisos() []Permiso { return []Permiso{PermisoExpedientePresentar} }

type fabricaRolCoordinador struct{}

func (fabricaRolCoordinador) CrearPerfil(string) PerfilRol { return nuevoPerfilCoordinador() }
func (fabricaRolCoordinador) Permisos() []Permiso          { return []Permiso{PermisoCoordinacionAcceder} }

func FabricaParaRol(rol Rol) (FabricaRol, error) {
	switch rol {
	case RolEscolar:
		return fabricaRolEscolar{}, nil
	case RolApoderado:
		return fabricaRolApoderado{}, nil
	case RolTutor:
		return fabricaRolTutor{}, nil
	case RolCoordinador:
		return fabricaRolCoordinador{}, nil
	default:
		return nil, ErrValidacion{Campos: map[string]string{"rol": "rol desconocido"}}
	}
}
