package acceso

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type RepositorioMemoria struct {
	mu                  sync.Mutex
	secuenciaID         int
	usuarios            map[string]*Usuario
	sesiones            map[string]Sesion
	tokens              map[string]*tokenMem
	tokensConfirmacion  map[string]*tokenConfirmacionMem
	vinculaciones       map[string]*Vinculacion
	autorizaciones      map[string]*Autorizacion
	ordenAutorizaciones []string
}

type tokenMem struct {
	TokenRecuperacion
	usado bool
}

type tokenConfirmacionMem struct {
	TokenConfirmacion
	usado bool
}

func NuevoRepositorioMemoria() *RepositorioMemoria {
	return &RepositorioMemoria{
		usuarios:           map[string]*Usuario{},
		sesiones:           map[string]Sesion{},
		tokens:             map[string]*tokenMem{},
		tokensConfirmacion: map[string]*tokenConfirmacionMem{},
		vinculaciones:      map[string]*Vinculacion{},
		autorizaciones:     map[string]*Autorizacion{},
	}
}

func (r *RepositorioMemoria) CrearUsuario(_ context.Context, usuario *Usuario) (*Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existente := range r.usuarios {
		if existente.Email == usuario.Email {
			return nil, ErrEmailEnUso
		}
	}
	r.secuenciaID++
	registrado := *usuario
	registrado.ID = fmt.Sprintf("u-%d", r.secuenciaID)
	r.usuarios[registrado.ID] = &registrado
	copia := registrado
	return &copia, nil
}

func (r *RepositorioMemoria) UsuarioPorEmail(_ context.Context, email string) (*Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, usuario := range r.usuarios {
		if usuario.Email == email {
			copia := *usuario
			return &copia, nil
		}
	}
	return nil, ErrNoEncontrado
}

func (r *RepositorioMemoria) UsuarioPorID(_ context.Context, id string) (*Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if usuario, ok := r.usuarios[id]; ok {
		copia := *usuario
		return &copia, nil
	}
	return nil, ErrNoEncontrado
}

func (r *RepositorioMemoria) ActualizarClave(_ context.Context, id, claveHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if usuario, ok := r.usuarios[id]; ok {
		usuario.ClaveHash = claveHash
		return nil
	}
	return ErrNoEncontrado
}

func (r *RepositorioMemoria) MarcarCorreoConfirmado(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if usuario, ok := r.usuarios[id]; ok {
		usuario.EmailConfirmado = true
		return nil
	}
	return ErrNoEncontrado
}

func (r *RepositorioMemoria) RegistrarIngreso(_ context.Context, id string, momento time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if usuario, ok := r.usuarios[id]; ok {
		usuario.UltimoIngreso = &momento
	}
	return nil
}

func (r *RepositorioMemoria) GuardarSesion(_ context.Context, sesion Sesion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sesiones[string(sesion.TokenHash)] = sesion
	return nil
}

func (r *RepositorioMemoria) SesionPorHash(_ context.Context, tokenHash []byte) (*Sesion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sesion, ok := r.sesiones[string(tokenHash)]; ok {
		return &sesion, nil
	}
	return nil, ErrNoEncontrado
}

func (r *RepositorioMemoria) EliminarSesion(_ context.Context, tokenHash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sesiones, string(tokenHash))
	return nil
}

func (r *RepositorioMemoria) EliminarSesionesDeUsuario(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for clave, sesion := range r.sesiones {
		if sesion.UsuarioID == id {
			delete(r.sesiones, clave)
		}
	}
	return nil
}

func (r *RepositorioMemoria) GuardarTokenRecuperacion(_ context.Context, token TokenRecuperacion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[string(token.TokenHash)] = &tokenMem{TokenRecuperacion: token}
	return nil
}

func (r *RepositorioMemoria) ConsumirTokenRecuperacion(_ context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token := r.tokens[string(tokenHash)]
	if !tokenVigente(token, ahora) {
		return "", ErrTokenInvalido
	}
	token.usado = true
	return token.UsuarioID, nil
}

func (r *RepositorioMemoria) UsuarioDeTokenRecuperacion(_ context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token := r.tokens[string(tokenHash)]
	if !tokenVigente(token, ahora) {
		return "", ErrTokenInvalido
	}
	return token.UsuarioID, nil
}

func (r *RepositorioMemoria) GuardarTokenConfirmacion(_ context.Context, token TokenConfirmacion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokensConfirmacion[string(token.TokenHash)] = &tokenConfirmacionMem{TokenConfirmacion: token}
	return nil
}

func (r *RepositorioMemoria) ConsumirTokenConfirmacion(_ context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token := r.tokensConfirmacion[string(tokenHash)]
	if token == nil || token.usado || !ahora.Before(token.ExpiraEn) {
		return "", ErrTokenInvalido
	}
	token.usado = true
	return token.UsuarioID, nil
}

// EliminarUsuariosSinConfirmar libera los correos de registros que nunca
// confirmaron: solo conserva las cuentas sin confirmar con algún enlace vivo.
func (r *RepositorioMemoria) EliminarUsuariosSinConfirmar(_ context.Context, ahora time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var liberados int64
	for id, usuario := range r.usuarios {
		if usuario.EmailConfirmado || r.tieneEnlaceVigente(id, ahora) {
			continue
		}
		for clave, token := range r.tokensConfirmacion {
			if token.UsuarioID == id {
				delete(r.tokensConfirmacion, clave)
			}
		}
		delete(r.usuarios, id)
		liberados++
	}
	return liberados, nil
}

func (r *RepositorioMemoria) tieneEnlaceVigente(usuarioID string, ahora time.Time) bool {
	for _, token := range r.tokensConfirmacion {
		if token.UsuarioID == usuarioID && !token.usado && ahora.Before(token.ExpiraEn) {
			return true
		}
	}
	return false
}

func tokenVigente(token *tokenMem, ahora time.Time) bool {
	return token != nil && !token.usado && ahora.Before(token.ExpiraEn)
}

func (r *RepositorioMemoria) GuardarSolicitudVinculacion(_ context.Context, vinculacion Vinculacion) (*Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	registrada := vinculacion
	r.secuenciaID++
	registrada.ID = fmt.Sprintf("v-%d", r.secuenciaID)
	if registrada.CreadaEn.IsZero() {
		registrada.CreadaEn = time.Now().UTC()
	}
	r.vinculaciones[registrada.ID] = &registrada
	return r.copiarVinculacion(&registrada), nil
}

func (r *RepositorioMemoria) SolicitudPendienteDeEscolar(_ context.Context, escolarID string, ahora time.Time) (*Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var encontrada *Vinculacion
	for _, vinculacion := range r.vinculaciones {
		if vinculacion.EscolarID != escolarID || vinculacion.Estado != EstadoVinculacionPendiente || !ahora.Before(vinculacion.ExpiraEn) {
			continue
		}
		if encontrada == nil || vinculacion.CreadaEn.After(encontrada.CreadaEn) {
			encontrada = vinculacion
		}
	}
	if encontrada == nil {
		return nil, ErrNoEncontrado
	}
	return r.copiarVinculacion(encontrada), nil
}

func (r *RepositorioMemoria) VinculacionAceptadaDeEscolar(_ context.Context, escolarID string) (*Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var encontrada *Vinculacion
	for _, vinculacion := range r.vinculaciones {
		if vinculacion.EscolarID != escolarID || vinculacion.Estado != EstadoVinculacionAceptada {
			continue
		}
		if encontrada == nil || vinculacion.CreadaEn.After(encontrada.CreadaEn) {
			encontrada = vinculacion
		}
	}
	if encontrada == nil {
		return nil, ErrNoEncontrado
	}
	return r.copiarVinculacion(encontrada), nil
}

func (r *RepositorioMemoria) ConsumirSolicitudVinculacion(_ context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, vinculacion := range r.vinculaciones {
		if vinculacion.EscolarID != escolarID || vinculacion.Estado != EstadoVinculacionPendiente ||
			!ahora.Before(vinculacion.ExpiraEn) || string(vinculacion.TokenHash) != string(tokenHash) {
			continue
		}
		vinculacion.Estado = EstadoVinculacionAceptada
		momento := ahora
		vinculacion.AceptadaEn = &momento
		r.secuenciaID++
		id := fmt.Sprintf("a-%d", r.secuenciaID)
		r.autorizaciones[id] = &Autorizacion{
			ID:            id,
			VinculacionID: vinculacion.ID,
			ApoderadoID:   vinculacion.ApoderadoID,
			EscolarID:     vinculacion.EscolarID,
			Estado:        AutorizacionOtorgada,
			CreadaEn:      momento,
		}
		r.ordenAutorizaciones = append(r.ordenAutorizaciones, id)
		return r.copiarVinculacion(vinculacion), nil
	}
	return nil, ErrTokenInvalido
}

func (r *RepositorioMemoria) VinculacionesDeApoderado(_ context.Context, apoderadoID string, ahora time.Time) ([]Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	vinculaciones := make([]Vinculacion, 0)
	for _, vinculacion := range r.vinculaciones {
		if vinculacion.ApoderadoID != apoderadoID {
			continue
		}
		if vinculacion.Estado == EstadoVinculacionPendiente && !ahora.Before(vinculacion.ExpiraEn) {
			continue
		}
		vinculaciones = append(vinculaciones, *r.copiarVinculacion(vinculacion))
	}
	sort.Slice(vinculaciones, func(i, j int) bool {
		if vinculaciones[i].Estado != vinculaciones[j].Estado {
			return vinculaciones[i].Estado == EstadoVinculacionAceptada
		}
		return vinculaciones[i].CreadaEn.After(vinculaciones[j].CreadaEn)
	})
	return vinculaciones, nil
}

func (r *RepositorioMemoria) copiarVinculacion(vinculacion *Vinculacion) *Vinculacion {
	copia := *vinculacion
	if usuario, ok := r.usuarios[copia.ApoderadoID]; ok {
		copia.ApoderadoNombre = usuario.NombreUso
		copia.ApoderadoEmail = usuario.Email
	}
	if usuario, ok := r.usuarios[copia.EscolarID]; ok {
		copia.EscolarNombre = usuario.NombreUso
		copia.EscolarEmail = usuario.Email
	}
	if autorizacion := r.ultimaAutorizacionDeVinculacion(copia.ID); autorizacion != nil {
		copia.Autorizacion = r.copiarAutorizacion(autorizacion)
	}
	return &copia
}

func (r *RepositorioMemoria) ultimaAutorizacionDeVinculacion(vinculacionID string) *Autorizacion {
	var encontrada *Autorizacion
	for _, autorizacion := range r.autorizaciones {
		if autorizacion.VinculacionID != vinculacionID {
			continue
		}
		if encontrada == nil || autorizacion.CreadaEn.After(encontrada.CreadaEn) ||
			(autorizacion.CreadaEn.Equal(encontrada.CreadaEn) && autorizacion.ID > encontrada.ID) {
			encontrada = autorizacion
		}
	}
	return encontrada
}

func (r *RepositorioMemoria) copiarAutorizacion(autorizacion *Autorizacion) *Autorizacion {
	copia := *autorizacion
	if usuario, ok := r.usuarios[copia.ApoderadoID]; ok {
		copia.ApoderadoNombre = usuario.NombreUso
		copia.ApoderadoEmail = usuario.Email
	}
	if usuario, ok := r.usuarios[copia.EscolarID]; ok {
		copia.EscolarNombre = usuario.NombreUso
		copia.EscolarEmail = usuario.Email
	}
	return &copia
}

func (r *RepositorioMemoria) AutorizacionVigenteDeEscolar(_ context.Context, escolarID string) (*Autorizacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var vinculacion *Vinculacion
	for _, candidata := range r.vinculaciones {
		if candidata.EscolarID == escolarID && candidata.Estado == EstadoVinculacionAceptada {
			vinculacion = candidata
			break
		}
	}
	if vinculacion == nil {
		return nil, ErrNoEncontrado
	}
	autorizacion := r.ultimaAutorizacionDeVinculacion(vinculacion.ID)
	if autorizacion == nil {
		return nil, ErrNoEncontrado
	}
	return r.copiarAutorizacion(autorizacion), nil
}

func (r *RepositorioMemoria) CambiarAutorizacion(_ context.Context, vinculacionID, escolarID, apoderadoID string, estado EstadoAutorizacion, ahora time.Time) (*Autorizacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	vinculacion, ok := r.vinculaciones[vinculacionID]
	if !ok || vinculacion.Estado != EstadoVinculacionAceptada ||
		vinculacion.EscolarID != escolarID || vinculacion.ApoderadoID != apoderadoID {
		return nil, ErrNoEncontrado
	}
	if actual := r.ultimaAutorizacionDeVinculacion(vinculacionID); actual != nil {
		if actual.Estado == estado {
			return r.copiarAutorizacion(actual), nil
		}
	}
	r.secuenciaID++
	autorizacion := &Autorizacion{
		ID:            fmt.Sprintf("a-%d", r.secuenciaID),
		VinculacionID: vinculacionID,
		ApoderadoID:   apoderadoID,
		EscolarID:     escolarID,
		Estado:        estado,
		CreadaEn:      ahora,
	}
	r.autorizaciones[autorizacion.ID] = autorizacion
	r.ordenAutorizaciones = append(r.ordenAutorizaciones, autorizacion.ID)
	return r.copiarAutorizacion(autorizacion), nil
}

// AutorizacionesDeVinculacion devuelve el historial completo del vínculo, del
// cambio más reciente al más antiguo. Cada fila es un cambio de permiso.
func (r *RepositorioMemoria) AutorizacionesDeVinculacion(_ context.Context, vinculacionID string) ([]Autorizacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	autorizaciones := make([]Autorizacion, 0)
	for i := len(r.ordenAutorizaciones) - 1; i >= 0; i-- {
		autorizacion := r.autorizaciones[r.ordenAutorizaciones[i]]
		if autorizacion == nil || autorizacion.VinculacionID != vinculacionID {
			continue
		}
		autorizaciones = append(autorizaciones, *r.copiarAutorizacion(autorizacion))
	}
	return autorizaciones, nil
}

func (r *RepositorioMemoria) SolicitudVinculacionPorToken(_ context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, vinculacion := range r.vinculaciones {
		if vinculacion.EscolarID == escolarID && vinculacion.Estado == EstadoVinculacionPendiente &&
			ahora.Before(vinculacion.ExpiraEn) && string(vinculacion.TokenHash) == string(tokenHash) {
			return r.copiarVinculacion(vinculacion), nil
		}
	}
	return nil, ErrTokenInvalido
}
