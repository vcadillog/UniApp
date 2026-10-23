package reserva

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Reglas del flujo D. Viven en el modelo, no en el controlador ni en la vista:
// RN-03 (48 horas), RN-04 (UTC y America/Lima), RN-05 (capacidad), RN-07 (datos de
// preparación), D05 (confirmación inmediata), D06 (60 minutos y 28 días) y D07 (equidad).
const (
	DuracionSesion     = 60 * time.Minute
	AnticipacionMinima = 48 * time.Hour
	HorizonteMaximo    = 28 * 24 * time.Hour

	TemaMinimo, TemaMaximo         = 10, 200
	ObjetivoMinimo, ObjetivoMaximo = 20, 1000
	GradoMaximo                    = 80

	MaxAdjuntos         = 5
	AdjuntoMaximoBytes  = 10 << 20
	NombreAdjuntoMaximo = 255

	// D07: cada escolar tiene como máximo dos reservas futuras activas y una sesión
	// por curso por semana.
	MaxFuturasPorEscolar = 2
	MaxPorCursoSemana    = 1
)

// ZonaLima es una zona fija de -05:00: el Perú no aplica horario de verano (RN-04).
// Se evita depender de la base de zonas horarias del sistema operativo.
var ZonaLima = time.FixedZone("America/Lima", -5*60*60)

type Estado string

const EstadoConfirmada Estado = "confirmada"

// Limites son los topes de equidad que el repositorio aplica dentro de la transacción.
// Se pasan como dato para que la regla siga viviendo en el modelo.
type Limites struct {
	MaxFuturas        int
	MaxPorCursoSemana int
}

var LimitesPorDefecto = Limites{MaxFuturas: MaxFuturasPorEscolar, MaxPorCursoSemana: MaxPorCursoSemana}

// Slot es un horario publicado por el flujo C (oferta). En S1 se consume desde una
// oferta sintética; cuando I08 publique la búsqueda se cambia solo el adaptador.
type Slot struct {
	ID          string
	TutorID     string
	TutorNombre string
	Curso       string
	Inicio      time.Time
	Fin         time.Time
}

// Disposicion indica si un slot puede reservarse y, si no, por qué. La pantalla la usa
// para deshabilitar opciones sin duplicar reglas en la vista.
type Disposicion struct {
	Slot   Slot
	Motivo string
}

func (d Disposicion) Disponible() bool { return d.Motivo == "" }

// Adjunto solo guarda metadatos; el contenido vive en el almacén privado y se abre por
// un enlace autenticado. Nunca se expone la clave de almacenamiento en la API.
type Adjunto struct {
	ID                  string
	Nombre              string
	Tipo                string
	TamanoBytes         int64
	Estado              EstadoAdjunto
	Motivo              string
	Huella              string
	ClaveAlmacenamiento string
	CreadoEn            time.Time
}

type Reserva struct {
	ID                string
	EscolarID         string
	TutorID           string
	TutorNombre       string
	SlotID            string
	Curso             string
	Grado             string
	Inicio            time.Time
	Fin               time.Time
	Tema              string
	Objetivo          string
	Estado            Estado
	ClaveIdempotencia string
	CreadaEn          time.Time
	Adjuntos          []Adjunto
}

// DatosReserva es lo que envía el formulario. El escolar nunca se recibe del cliente:
// sale de la sesión autenticada. Los adjuntos se suben después de crear la reserva
// (I09), para que un fallo de análisis no tumbe la reserva.
type DatosReserva struct {
	SlotID            string
	Tema              string
	Objetivo          string
	Grado             string
	ClaveIdempotencia string
}

var (
	ErrSlotInexistente       = errors.New("el horario elegido no existe")
	ErrSlotVencido           = errors.New("el horario elegido ya pasó")
	ErrSlotOcupado           = errors.New("el horario elegido ya fue reservado")
	ErrSolapamiento          = errors.New("ya tienes una sesión que se cruza con ese horario")
	ErrCuotaEscolar          = errors.New("alcanzaste el máximo de reservas futuras activas")
	ErrCuotaCursoSemana      = errors.New("ya tienes una sesión de este curso en esa semana")
	ErrReservaNoEncontrada   = errors.New("reserva no encontrada")
	ErrNoAutorizado          = errors.New("no autorizado")
	ErrSinAutorizacion       = errors.New("el escolar no tiene autorización vigente para reservar")
	ErrAnticipacion          = errors.New("la sesión debe empezar al menos 48 horas después de este momento")
	ErrHorizonte             = errors.New("el horario está fuera del horizonte de 28 días")
	ErrReservaInvalida       = errors.New("reserva inválida")
	ErrIdempotenciaConflicto = errors.New("la clave de idempotencia ya se usó con otros datos")
	ErrAdjuntosLlenos        = errors.New("la reserva ya tiene el máximo de adjuntos")
	ErrAdjuntoNoEncontrado   = errors.New("adjunto no encontrado")
	ErrAdjuntoEnCuarentena   = errors.New("el adjunto sigue en cuarentena y todavía no se puede descargar")
	ErrAdjuntoRechazado      = errors.New("el adjunto fue rechazado por el análisis")
	ErrAdjuntoInvalido       = errors.New("adjunto inválido")
	ErrAlmacen               = errors.New("no se pudo guardar el adjunto")
)

// ErrValidacion agrupa los errores por campo para que la vista los muestre sin repetir
// las reglas. Es propio del flujo D para no acoplar la reserva al modelo de acceso.
type ErrValidacion struct{ Campos map[string]string }

func (e ErrValidacion) Error() string { return "datos inválidos" }

// MotivoPorTiempo devuelve "" si el slot cumple la ventana de reserva, o el motivo por
// el que no puede reservarse todavía. Es la única fuente de RN-03 en el proyecto.
func MotivoPorTiempo(slot Slot, ahora time.Time) string {
	if err := validarVentana(slot, ahora); err != nil {
		return err.Error()
	}
	return ""
}

func validarVentana(slot Slot, ahora time.Time) error {
	ahora = ahora.UTC()
	switch {
	case !slot.Fin.After(ahora):
		return ErrSlotVencido
	case slot.Inicio.Sub(ahora) < AnticipacionMinima:
		return ErrAnticipacion
	case slot.Inicio.Sub(ahora) > HorizonteMaximo:
		return ErrHorizonte
	}
	return nil
}

// SemanaLima devuelve el intervalo [lunes 00:00, lunes siguiente 00:00) en America/Lima,
// expresado en UTC. D06 fija la semana así y RN-05 cuenta consumos dentro de ella.
func SemanaLima(momento time.Time) (time.Time, time.Time) {
	local := momento.In(ZonaLima)
	dia := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, ZonaLima)
	desplazamiento := (int(dia.Weekday()) + 6) % 7 // lunes = 0
	inicio := dia.AddDate(0, 0, -desplazamiento)
	return inicio.UTC(), inicio.AddDate(0, 0, 7).UTC()
}

func normalizarDatos(datos DatosReserva) DatosReserva {
	datos.SlotID = strings.TrimSpace(datos.SlotID)
	datos.Tema = strings.TrimSpace(datos.Tema)
	datos.Objetivo = strings.TrimSpace(datos.Objetivo)
	datos.Grado = strings.TrimSpace(datos.Grado)
	datos.ClaveIdempotencia = strings.TrimSpace(datos.ClaveIdempotencia)
	return datos
}

func validarDatos(datos DatosReserva) map[string]string {
	campos := map[string]string{}
	if datos.SlotID == "" {
		campos["slot_id"] = "elige un horario disponible"
	}
	if longitud := utf8.RuneCountInString(datos.Tema); longitud < TemaMinimo || longitud > TemaMaximo {
		campos["tema"] = "el tema debe tener entre 10 y 200 caracteres"
	}
	if longitud := utf8.RuneCountInString(datos.Objetivo); longitud < ObjetivoMinimo || longitud > ObjetivoMaximo {
		campos["objetivo"] = "el objetivo debe tener entre 20 y 1000 caracteres"
	}
	if longitud := utf8.RuneCountInString(datos.Grado); longitud < 1 || longitud > GradoMaximo {
		campos["grado"] = "indica tu grado (por ejemplo, 5to de secundaria)"
	}
	if datos.ClaveIdempotencia != "" {
		if mensaje := validarClaveIdempotencia(datos.ClaveIdempotencia); mensaje != "" {
			campos["idempotency_key"] = mensaje
		}
	}
	return campos
}

func validarClaveIdempotencia(clave string) string {
	if len(clave) < 8 || len(clave) > 128 {
		return "la clave debe tener entre 8 y 128 caracteres"
	}
	for _, caracter := range clave {
		switch {
		case caracter >= 'a' && caracter <= 'z', caracter >= 'A' && caracter <= 'Z',
			caracter >= '0' && caracter <= '9', caracter == '-', caracter == '_':
		default:
			return "la clave solo admite letras, números, guion y guion bajo"
		}
	}
	return ""
}

// NuevaReserva aplica RN-03, RN-07 y D05 (la reserva válida se confirma de inmediato).
func NuevaReserva(escolarID string, datos DatosReserva, slot Slot, ahora time.Time) (*Reserva, error) {
	if strings.TrimSpace(escolarID) == "" {
		return nil, ErrNoAutorizado
	}
	datos = normalizarDatos(datos)
	if campos := validarDatos(datos); len(campos) > 0 {
		return nil, ErrValidacion{Campos: campos}
	}
	if slot.ID == "" || slot.TutorID == "" || !slot.Fin.After(slot.Inicio) {
		return nil, ErrSlotInexistente
	}
	if err := validarVentana(slot, ahora); err != nil {
		return nil, err
	}
	return &Reserva{
		EscolarID:         escolarID,
		TutorID:           slot.TutorID,
		TutorNombre:       slot.TutorNombre,
		SlotID:            slot.ID,
		Curso:             slot.Curso,
		Grado:             datos.Grado,
		Inicio:            slot.Inicio.UTC(),
		Fin:               slot.Fin.UTC(),
		Tema:              datos.Tema,
		Objetivo:          datos.Objetivo,
		Estado:            EstadoConfirmada,
		ClaveIdempotencia: datos.ClaveIdempotencia,
		CreadaEn:          ahora.UTC(),
	}, nil
}

// MismosDatos compara la reserva pedida con la ya guardada para decidir si la clave de
// idempotencia es un reintento legítimo o un uso indebido con otro contenido.
func MismosDatos(guardada *Reserva, pedida *Reserva) bool {
	return guardada.SlotID == pedida.SlotID &&
		guardada.Tema == pedida.Tema &&
		guardada.Objetivo == pedida.Objetivo &&
		guardada.Grado == pedida.Grado
}
