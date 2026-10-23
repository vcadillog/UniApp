package acceso

import (
	"strings"
)

type PerfilRol interface {
	Rol() Rol
	Politica() PoliticaClave
	EsPublico() bool

	ValidarExtra(d DatosRegistro) map[string]string
}

type perfilBase struct {
	rol      Rol
	politica PoliticaClave
	publico  bool
}

func (p perfilBase) Rol() Rol                                     { return p.rol }
func (p perfilBase) Politica() PoliticaClave                      { return p.politica }
func (p perfilBase) EsPublico() bool                              { return p.publico }
func (p perfilBase) ValidarExtra(DatosRegistro) map[string]string { return nil }

type perfilEscolar struct{ perfilBase }

func nuevoPerfilEscolar() PerfilRol {
	return &perfilEscolar{perfilBase{rol: RolEscolar, politica: PoliticaBasica{Min: 10}, publico: true}}
}

type perfilApoderado struct{ perfilBase }

func nuevoPerfilApoderado() PerfilRol {
	return &perfilApoderado{perfilBase{rol: RolApoderado, politica: PoliticaBasica{Min: 10}, publico: true}}
}

type perfilTutor struct {
	perfilBase
	sufijos []string
}

func nuevoPerfilTutor(dominios string) PerfilRol {
	return &perfilTutor{
		perfilBase: perfilBase{rol: RolTutor, politica: PoliticaEstricta{Min: 12}, publico: true},
		sufijos:    sufijosDeDominios(dominios),
	}
}

func (p *perfilTutor) ValidarExtra(d DatosRegistro) map[string]string {
	for _, sufijo := range p.sufijos {
		if strings.HasSuffix(d.Email, sufijo) {
			return nil
		}
	}
	if len(p.sufijos) == 0 {
		return map[string]string{"email": "los tutores usan su correo institucional"}
	}
	return map[string]string{"email": "los tutores usan su correo institucional (" + strings.Join(p.sufijos, " o ") + ")"}
}

func dominiosDe(texto string) []string {
	dominios := make([]string, 0, 2)
	for _, parte := range strings.Split(texto, ",") {
		if dominio := strings.ToLower(strings.TrimSpace(parte)); dominio != "" {
			dominios = append(dominios, dominio)
		}
	}
	return dominios
}

func sufijosDeDominios(texto string) []string {
	dominios := dominiosDe(texto)
	sufijos := make([]string, 0, len(dominios))
	for _, dominio := range dominios {
		sufijos = append(sufijos, "@"+dominio)
	}
	return sufijos
}

type perfilCoordinador struct{ perfilBase }

func nuevoPerfilCoordinador() PerfilRol {
	return &perfilCoordinador{perfilBase{rol: RolCoordinador, politica: PoliticaEstricta{Min: 12}, publico: false}}
}

func ObtenerPerfil(rol Rol, dominiosTutor string) (PerfilRol, error) {
	fabrica, err := FabricaParaRol(rol)
	if err != nil {
		return nil, err
	}
	perfil := fabrica.CrearPerfil(dominiosTutor)
	if perfil == nil {
		return nil, ErrValidacion{Campos: map[string]string{"rol": "rol desconocido"}}
	}
	return perfil, nil
}
