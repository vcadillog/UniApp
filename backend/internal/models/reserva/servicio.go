package reserva

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	modeloAcceso "uniapp/internal/models/acceso"
)

// Servicio aplica las reglas del flujo D. El reloj es inyectable para probar las 48 horas
// sin esperar: el servidor es la autoridad temporal (RN-03).
type Servicio struct {
	repositorio  Repositorio
	oferta       PuertoOferta
	autorizacion PuertoAutorizacion
	almacen      Almacen
	analizador   Analizador
	limites      Limites
	ahora        func() time.Time
}

// PuertoAutorizacion consulta el flujo A sin escribir en sus tablas.
type PuertoAutorizacion interface {
	AutorizacionVigente(ctx context.Context, escolarID string) (bool, error)
}

type Dependencias struct {
	Repositorio  Repositorio
	Oferta       PuertoOferta
	Autorizacion PuertoAutorizacion
	Almacen      Almacen
	Analizador   Analizador
	Limites      Limites
}

func NuevoServicio(dependencias Dependencias) *Servicio {
	limites := dependencias.Limites
	if limites.MaxFuturas == 0 && limites.MaxPorCursoSemana == 0 {
		limites = LimitesPorDefecto
	}
	analizador := dependencias.Analizador
	if analizador == nil {
		analizador = AnalizadorNoDisponible{}
	}
	autorizacion := dependencias.Autorizacion
	if autorizacion == nil {
		// Sin un puerto inyectado no se puede demostrar autorización: se falla
		// cerrado. El composition root siempre inyecta el flujo A.
		autorizacion = autorizacionNula{}
	}
	return &Servicio{
		repositorio:  dependencias.Repositorio,
		oferta:       dependencias.Oferta,
		autorizacion: autorizacion,
		almacen:      dependencias.Almacen,
		analizador:   analizador,
		limites:      limites,
		ahora:        func() time.Time { return time.Now().UTC() },
	}
}

type autorizacionNula struct{}

func (autorizacionNula) AutorizacionVigente(context.Context, string) (bool, error) {
	return false, nil
}

func (s *Servicio) FijarReloj(f func() time.Time) { s.ahora = f }

func (s *Servicio) Ahora() time.Time { return s.ahora().UTC() }

// Slots devuelve la oferta con el motivo por el que cada horario no puede reservarse.
// La vista solo pinta el resultado; no recalcula las 48 horas.
func (s *Servicio) Slots(ctx context.Context) ([]Disposicion, error) {
	slots, err := s.oferta.Slots(ctx)
	if err != nil {
		return nil, err
	}
	ahora := s.Ahora()
	disposiciones := make([]Disposicion, 0, len(slots))
	for _, slot := range slots {
		motivo := MotivoPorTiempo(slot, ahora)
		if motivo == "" {
			tomado, err := s.repositorio.SlotTomado(ctx, slot.ID)
			if err != nil {
				return nil, err
			}
			if tomado {
				motivo = ErrSlotOcupado.Error()
			}
		}
		disposiciones = append(disposiciones, Disposicion{Slot: slot, Motivo: motivo})
	}
	return disposiciones, nil
}

// Reservar devuelve creada=false cuando la clave de idempotencia repite una reserva ya
// existente con los mismos datos (reintento seguro del formulario).
func (s *Servicio) Reservar(ctx context.Context, escolarID string, datos DatosReserva) (*Reserva, bool, error) {
	if strings.TrimSpace(escolarID) == "" {
		return nil, false, ErrNoAutorizado
	}
	// La autorización se comprueba solo al crear: las reservas ya confirmadas
	// siguen vigentes aunque el apoderado revoque el permiso después.
	autorizado, err := s.autorizacion.AutorizacionVigente(ctx, escolarID)
	if err != nil {
		return nil, false, err
	}
	if !autorizado {
		return nil, false, ErrSinAutorizacion
	}
	slot, err := s.slotElegido(ctx, datos.SlotID)
	if err != nil {
		return nil, false, err
	}
	reserva, err := NuevaReserva(escolarID, datos, slot, s.Ahora())
	if err != nil {
		return nil, false, err
	}
	if reserva.ClaveIdempotencia != "" {
		guardada, err := s.repositorio.PorClaveIdempotencia(ctx, escolarID, reserva.ClaveIdempotencia)
		switch {
		case err == nil:
			if !MismosDatos(guardada, reserva) {
				return nil, false, ErrIdempotenciaConflicto
			}
			return guardada, false, nil
		case !errors.Is(err, ErrReservaNoEncontrada):
			return nil, false, err
		}
	}
	return s.repositorio.ReservarTransaccional(ctx, reserva, s.limites)
}

func (s *Servicio) slotElegido(ctx context.Context, slotID string) (Slot, error) {
	if slotID == "" {
		return Slot{}, ErrValidacion{Campos: map[string]string{"slot_id": "elige un horario disponible"}}
	}
	return s.oferta.SlotPorID(ctx, slotID)
}

// ReservaDe solo devuelve la reserva a su dueño: otra persona recibe ErrNoAutorizado.
func (s *Servicio) ReservaDe(ctx context.Context, id, escolarID string) (*Reserva, error) {
	reserva, err := s.repositorio.PorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if reserva.EscolarID != escolarID {
		return nil, ErrNoAutorizado
	}
	return reserva, nil
}

func (s *Servicio) ReservasDe(ctx context.Context, escolarID string) ([]Reserva, error) {
	return s.repositorio.PorEscolar(ctx, escolarID)
}

// Adjuntar guarda un archivo en cuarentena. Si el análisis no está disponible, el adjunto
// queda pendiente y la reserva sigue confirmada: I09 exige que el fallo del análisis no
// tumbe la reserva.
func (s *Servicio) Adjuntar(ctx context.Context, usuarioID string, reservaID, nombre string, contenido []byte) (*Adjunto, error) {
	reserva, err := s.repositorio.PorID(ctx, reservaID)
	if err != nil {
		return nil, err
	}
	if reserva.EscolarID != usuarioID {
		return nil, ErrNoAutorizado
	}
	if s.almacen == nil {
		return nil, ErrAlmacen
	}
	total, err := s.repositorio.ContarAdjuntos(ctx, reservaID)
	if err != nil {
		return nil, err
	}
	if total >= MaxAdjuntos {
		return nil, ErrAdjuntosLlenos
	}
	if err := ValidarEntradaAdjunto(nombre, int64(len(contenido))); err != nil {
		return nil, err
	}
	tipo := TipoDetectado(contenido)
	if _, permitido := ExtensionPermitida(tipo); !permitido {
		return nil, ErrAdjuntoInvalido
	}
	clave, err := nuevaClaveAlmacen()
	if err != nil {
		return nil, err
	}
	if err := s.almacen.Guardar(ctx, clave, contenido); err != nil {
		return nil, ErrAlmacen
	}
	estado, motivo, errAnalisis := s.analizador.Analizar(nombre, tipo, contenido)
	if errors.Is(errAnalisis, ErrAnalisisNoDisponible) {
		estado = AdjuntoEnCuarentena
	} else if errAnalisis != nil {
		estado, motivo = AdjuntoEnCuarentena, "pendiente de análisis: "+errAnalisis.Error()
	}
	adjunto, err := NuevoAdjunto(nombre, tipo, int64(len(contenido)), HuellaAdjunto(contenido), clave, estado, motivo, s.Ahora())
	if err != nil {
		_ = s.almacen.Eliminar(ctx, clave)
		return nil, err
	}
	guardado, err := s.repositorio.AgregarAdjunto(ctx, reservaID, adjunto)
	if err != nil {
		_ = s.almacen.Eliminar(ctx, clave)
		return nil, err
	}
	if guardado.Estado == AdjuntoRechazado {
		// Un archivo rechazado no se conserva: solo queda el metadato y el motivo.
		_ = s.almacen.Eliminar(ctx, clave)
		guardado.ClaveAlmacenamiento = ""
	}
	return guardado, nil
}

func (s *Servicio) AdjuntosDe(ctx context.Context, usuarioID string, rol modeloAcceso.Rol, reservaID string) ([]Adjunto, error) {
	reserva, err := s.reservaVisible(ctx, reservaID, usuarioID, rol)
	if err != nil {
		return nil, err
	}
	return s.repositorio.AdjuntosDe(ctx, reserva.ID)
}

// AbrirAdjunto entrega el contenido solo a los participantes de la reserva y solo si el
// adjunto salió de cuarentena. El enlace se sirve autenticado, nunca público.
func (s *Servicio) AbrirAdjunto(ctx context.Context, usuarioID string, rol modeloAcceso.Rol, reservaID, adjuntoID string) (*Adjunto, []byte, error) {
	reserva, err := s.reservaVisible(ctx, reservaID, usuarioID, rol)
	if err != nil {
		return nil, nil, err
	}
	adjunto, err := s.repositorio.AdjuntoPorID(ctx, reserva.ID, adjuntoID)
	if err != nil {
		return nil, nil, err
	}
	switch adjunto.Estado {
	case AdjuntoEnCuarentena:
		return nil, nil, ErrAdjuntoEnCuarentena
	case AdjuntoRechazado:
		return nil, nil, ErrAdjuntoRechazado
	}
	if s.almacen == nil {
		return nil, nil, ErrAlmacen
	}
	contenido, err := s.almacen.Leer(ctx, adjunto.ClaveAlmacenamiento)
	if err != nil {
		return nil, nil, ErrAlmacen
	}
	return adjunto, contenido, nil
}

func (s *Servicio) reservaVisible(ctx context.Context, reservaID, usuarioID string, rol modeloAcceso.Rol) (*Reserva, error) {
	reserva, err := s.repositorio.PorID(ctx, reservaID)
	if err != nil {
		return nil, err
	}
	if reserva.EscolarID == usuarioID || reserva.TutorID == usuarioID || rol == modeloAcceso.RolCoordinador {
		return reserva, nil
	}
	return nil, ErrNoAutorizado
}

func nuevaClaveAlmacen() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", ErrAlmacen
	}
	return hex.EncodeToString(bytes), nil
}
