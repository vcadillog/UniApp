package acceso

type Permiso string

const (
	PermisoCoordinacionAcceder Permiso = "coordinacion:acceder"
	PermisoExpedientePresentar Permiso = "expediente:presentar"
	PermisoReservaCrear        Permiso = "reserva:crear"
	PermisoAutorizacionOtorgar Permiso = "autorizacion:otorgar"
)

func (r Rol) Permisos() []Permiso {
	fabrica, err := FabricaParaRol(r)
	if err != nil {
		return nil
	}
	propios := fabrica.Permisos()
	copia := make([]Permiso, len(propios))
	copy(copia, propios)
	return copia
}

func (r Rol) TienePermiso(permiso Permiso) bool {
	fabrica, err := FabricaParaRol(r)
	if err != nil {
		return false
	}
	for _, propio := range fabrica.Permisos() {
		if propio == permiso {
			return true
		}
	}
	return false
}
