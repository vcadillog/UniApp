package reserva

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Reloj fijo: las pruebas de 48 horas no dependen de la hora real (RN-03).
var ahora = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func datosValidos() DatosReserva {
	return DatosReserva{
		SlotID:   "slot-1",
		Tema:     "Repaso de factorización",
		Objetivo: "Resolver ejercicios de factorización de trinomios sin ayuda del tutor.",
		Grado:    "5to de secundaria",
	}
}

func slotCurso(id, curso string, inicio time.Time) Slot {
	return Slot{
		ID:          id,
		TutorID:     TutorSinteticoID,
		TutorNombre: TutorSinteticoNombre,
		Curso:       curso,
		Inicio:      inicio,
		Fin:         inicio.Add(DuracionSesion),
	}
}

func slotEn(id string, inicio time.Time) Slot { return slotCurso(id, "Álgebra", inicio) }

type ofertaFija struct{ slots []Slot }

func (o ofertaFija) Slots(context.Context) ([]Slot, error) { return o.slots, nil }

func (o ofertaFija) SlotPorID(_ context.Context, id string) (Slot, error) {
	for _, slot := range o.slots {
		if slot.ID == id {
			return slot, nil
		}
	}
	return Slot{}, ErrSlotInexistente
}

type almacenMemoria struct {
	archivos map[string][]byte
}

func nuevoAlmacen() *almacenMemoria { return &almacenMemoria{archivos: map[string][]byte{}} }

func (a *almacenMemoria) Guardar(_ context.Context, clave string, contenido []byte) error {
	a.archivos[clave] = append([]byte(nil), contenido...)
	return nil
}

func (a *almacenMemoria) Leer(_ context.Context, clave string) ([]byte, error) {
	contenido, existe := a.archivos[clave]
	if !existe {
		return nil, ErrAlmacen
	}
	return contenido, nil
}

func (a *almacenMemoria) Eliminar(_ context.Context, clave string) error {
	delete(a.archivos, clave)
	return nil
}

type autorizacionFija bool

func (a autorizacionFija) AutorizacionVigente(context.Context, string) (bool, error) {
	return bool(a), nil
}

func servicioDePrueba(slots ...Slot) (*Servicio, *almacenMemoria) {
	almacen := nuevoAlmacen()
	servicio := NuevoServicio(Dependencias{
		Repositorio:  NuevoRepositorioMemoria(),
		Oferta:       ofertaFija{slots: slots},
		Autorizacion: autorizacionFija(true),
		Almacen:      almacen,
		Analizador:   AnalizadorBasico{},
	})
	servicio.FijarReloj(func() time.Time { return ahora })
	return servicio, almacen
}

func TestReservarSoloConAutorizacionVigente(t *testing.T) {
	servicio := NuevoServicio(Dependencias{
		Repositorio:  NuevoRepositorioMemoria(),
		Oferta:       NuevaOfertaSintetica(ahora),
		Autorizacion: autorizacionFija(false),
	})
	servicio.FijarReloj(func() time.Time { return ahora })
	if _, _, err := servicio.Reservar(context.Background(), "escolar-1", datosValidos()); !errors.Is(err, ErrSinAutorizacion) {
		t.Fatalf("sin autorización no se reserva: %v", err)
	}
}

func TestReservarSinEscolarAutenticado(t *testing.T) {
	servicio, _ := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	if _, _, err := servicio.Reservar(context.Background(), "  ", datosValidos()); !errors.Is(err, ErrNoAutorizado) {
		t.Fatalf("sin escolar autenticado no hay reserva: %v", err)
	}
}

func TestAnticipacionExactaDe48Horas(t *testing.T) {
	casos := []struct {
		nombre string
		inicio time.Time
		espera error
	}{
		{"exactamente 48 h", ahora.Add(48 * time.Hour), nil},
		{"un nanosegundo menos", ahora.Add(48*time.Hour - time.Nanosecond), ErrAnticipacion},
		{"49 horas", ahora.Add(49 * time.Hour), nil},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := NuevaReserva("escolar-1", datosValidos(), slotEn("slot-1", caso.inicio), ahora)
			if caso.espera == nil && err != nil {
				t.Fatalf("debe aceptar: %v", err)
			}
			if caso.espera != nil && !errors.Is(err, caso.espera) {
				t.Fatalf("debe rechazar con %v: %v", caso.espera, err)
			}
		})
	}
}

func TestHorizonteDe28Dias(t *testing.T) {
	if _, err := NuevaReserva("e", datosValidos(), slotEn("slot-1", ahora.Add(28*24*time.Hour)), ahora); err != nil {
		t.Fatalf("el borde del horizonte es válido: %v", err)
	}
	if _, err := NuevaReserva("e", datosValidos(), slotEn("slot-1", ahora.Add(29*24*time.Hour)), ahora); !errors.Is(err, ErrHorizonte) {
		t.Fatalf("fuera del horizonte: %v", err)
	}
}

func TestSlotVencido(t *testing.T) {
	if _, err := NuevaReserva("e", datosValidos(), slotEn("slot-1", ahora.Add(-72*time.Hour)), ahora); !errors.Is(err, ErrSlotVencido) {
		t.Fatalf("un horario pasado no se reserva: %v", err)
	}
}

func TestValidacionDeDatosPreparacion(t *testing.T) {
	casos := []struct {
		nombre string
		cambio func(*DatosReserva)
		campo  string
	}{
		{"tema corto", func(d *DatosReserva) { d.Tema = "corto" }, "tema"},
		{"tema largo", func(d *DatosReserva) { d.Tema = strings.Repeat("a", 201) }, "tema"},
		{"objetivo corto", func(d *DatosReserva) { d.Objetivo = "muy corto" }, "objetivo"},
		{"objetivo largo", func(d *DatosReserva) { d.Objetivo = strings.Repeat("a", 1001) }, "objetivo"},
		{"grado vacío", func(d *DatosReserva) { d.Grado = "  " }, "grado"},
		{"sin slot", func(d *DatosReserva) { d.SlotID = "" }, "slot_id"},
		{"clave idempotente con espacios", func(d *DatosReserva) { d.ClaveIdempotencia = "clave con espacios" }, "idempotency_key"},
		{"clave idempotente corta", func(d *DatosReserva) { d.ClaveIdempotencia = "corta" }, "idempotency_key"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			datos := datosValidos()
			caso.cambio(&datos)
			var errValidacion ErrValidacion
			_, err := NuevaReserva("escolar-1", datos, slotEn("slot-1", ahora.Add(72*time.Hour)), ahora)
			if !errors.As(err, &errValidacion) {
				t.Fatalf("se esperaba ErrValidacion: %v", err)
			}
			if errValidacion.Campos[caso.campo] == "" {
				t.Fatalf("falta el campo %q en %v", caso.campo, errValidacion.Campos)
			}
		})
	}
}

func TestSemanaLimaEmpiezaElLunes(t *testing.T) {
	// Miércoles 21/10/2026 10:00 en Lima = 15:00 UTC.
	miercoles := time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)
	inicio, fin := SemanaLima(miercoles)
	if inicio.Weekday() != time.Monday || fin.Sub(inicio) != 7*24*time.Hour {
		t.Fatalf("semana de Lima: %v - %v", inicio, fin)
	}
	if presentar := inicio.In(ZonaLima); presentar.Hour() != 0 || presentar.Day() != 19 || presentar.Month() != 10 {
		t.Fatalf("la semana debe empezar el lunes 19 a las 00:00 de Lima: %v", presentar)
	}
}

func TestReservaConClaveIdempotente(t *testing.T) {
	servicio, _ := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	datos := datosValidos()
	datos.ClaveIdempotencia = "formulario-2026-10-20-abc"

	primera, creada, err := servicio.Reservar(context.Background(), "escolar-1", datos)
	if err != nil || !creada {
		t.Fatalf("primera reserva: %v creada=%v", err, creada)
	}
	segunda, creada, err := servicio.Reservar(context.Background(), "escolar-1", datos)
	if err != nil || creada || segunda.ID != primera.ID {
		t.Fatalf("el reintento debe devolver la misma reserva: %v creada=%v", err, creada)
	}
	otro := datos
	otro.Tema = "Otro tema distinto para la sesión"
	if _, _, err := servicio.Reservar(context.Background(), "escolar-1", otro); !errors.Is(err, ErrIdempotenciaConflicto) {
		t.Fatalf("la misma clave con otros datos debe fallar: %v", err)
	}
}

func TestCuotaDeReservasFuturas(t *testing.T) {
	servicio, _ := servicioDePrueba(
		slotCurso("slot-1", "Álgebra", ahora.Add(72*time.Hour)),
		slotCurso("slot-2", "Aritmética", ahora.Add(96*time.Hour)),
		slotCurso("slot-3", "Geometría", ahora.Add(120*time.Hour)),
	)
	ctx := context.Background()
	for _, id := range []string{"slot-1", "slot-2"} {
		datos := datosValidos()
		datos.SlotID = id
		datos.Tema = "Tema " + id + " de la sesión"
		if _, _, err := servicio.Reservar(ctx, "escolar-1", datos); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	datos := datosValidos()
	datos.SlotID = "slot-3"
	if _, _, err := servicio.Reservar(ctx, "escolar-1", datos); !errors.Is(err, ErrCuotaEscolar) {
		t.Fatalf("la tercera reserva futura debe rechazarse: %v", err)
	}
}

func TestUnaSesionPorCursoEnLaSemana(t *testing.T) {
	servicio, _ := servicioDePrueba(
		slotEn("slot-1", ahora.Add(72*time.Hour)),
		slotEn("slot-2", ahora.Add(96*time.Hour)),
	)
	ctx := context.Background()
	if _, _, err := servicio.Reservar(ctx, "escolar-1", datosValidos()); err != nil {
		t.Fatal(err)
	}
	datos := datosValidos()
	datos.SlotID = "slot-2"
	datos.Tema = "Segunda sesión del mismo curso"
	if _, _, err := servicio.Reservar(ctx, "escolar-1", datos); !errors.Is(err, ErrCuotaCursoSemana) {
		t.Fatalf("una sesión por curso y semana: %v", err)
	}
	if _, _, err := servicio.Reservar(ctx, "escolar-2", datos); err != nil {
		t.Fatalf("otro escolar no comparte la cuota: %v", err)
	}
}

func TestSolapamientoDelEscolar(t *testing.T) {
	inicio := ahora.Add(72 * time.Hour)
	servicio, _ := servicioDePrueba(
		slotEn("slot-1", inicio),
		slotEn("slot-2", inicio.Add(30*time.Minute)),
	)
	ctx := context.Background()
	if _, _, err := servicio.Reservar(ctx, "escolar-1", datosValidos()); err != nil {
		t.Fatal(err)
	}
	datos := datosValidos()
	datos.SlotID = "slot-2"
	datos.Tema = "Sesión que se cruza con la anterior"
	if _, _, err := servicio.Reservar(ctx, "escolar-1", datos); !errors.Is(err, ErrSolapamiento) {
		t.Fatalf("horarios cruzados del mismo escolar: %v", err)
	}
}

func pdfDePrueba() []byte {
	return []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
}

func TestAdjuntarYDescargar(t *testing.T) {
	servicio, almacen := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	ctx := context.Background()
	reserva, _, err := servicio.Reservar(ctx, "escolar-1", datosValidos())
	if err != nil {
		t.Fatal(err)
	}
	adjunto, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "practica.pdf", pdfDePrueba())
	if err != nil {
		t.Fatal(err)
	}
	if adjunto.Estado != AdjuntoValidado || adjunto.Tipo != "application/pdf" || adjunto.Huella == "" {
		t.Fatalf("adjunto validado con metadatos: %+v", adjunto)
	}
	if len(almacen.archivos) != 1 {
		t.Fatalf("el contenido debe quedar en el almacén: %d", len(almacen.archivos))
	}
	descargado, contenido, err := servicio.AbrirAdjunto(ctx, "escolar-1", "escolar", reserva.ID, adjunto.ID)
	if err != nil || descargado.ID != adjunto.ID || string(contenido) != string(pdfDePrueba()) {
		t.Fatalf("descarga del dueño: %v", err)
	}
}

func TestAdjuntoQuedaEnCuarentenaSinAnalisis(t *testing.T) {
	almacen := nuevoAlmacen()
	servicio := NuevoServicio(Dependencias{
		Repositorio:  NuevoRepositorioMemoria(),
		Oferta:       ofertaFija{slots: []Slot{slotEn("slot-1", ahora.Add(72*time.Hour))}},
		Autorizacion: autorizacionFija(true),
		Almacen:      almacen,
		Analizador:   AnalizadorNoDisponible{},
	})
	servicio.FijarReloj(func() time.Time { return ahora })
	ctx := context.Background()
	reserva, _, err := servicio.Reservar(ctx, "escolar-1", datosValidos())
	if err != nil {
		t.Fatalf("la reserva no depende del análisis: %v", err)
	}
	adjunto, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "practica.pdf", pdfDePrueba())
	if err != nil {
		t.Fatalf("subir adjunto sin análisis no debe fallar: %v", err)
	}
	if adjunto.Estado != AdjuntoEnCuarentena || adjunto.Motivo == "" {
		t.Fatalf("debe quedar en cuarentena con motivo: %+v", adjunto)
	}
	if _, _, err := servicio.AbrirAdjunto(ctx, "escolar-1", "escolar", reserva.ID, adjunto.ID); !errors.Is(err, ErrAdjuntoEnCuarentena) {
		t.Fatalf("no se descarga en cuarentena: %v", err)
	}
}

func TestAdjuntoRechazadoNoSeConserva(t *testing.T) {
	servicio, almacen := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	ctx := context.Background()
	reserva, _, _ := servicio.Reservar(ctx, "escolar-1", datosValidos())
	eicar := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")
	adjunto, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "prueba.txt", eicar)
	if err != nil {
		t.Fatal(err)
	}
	if adjunto.Estado != AdjuntoRechazado || adjunto.Motivo == "" {
		t.Fatalf("debe rechazarse con motivo: %+v", adjunto)
	}
	if len(almacen.archivos) != 0 {
		t.Fatal("un adjunto rechazado no conserva el contenido")
	}
	if _, _, err := servicio.AbrirAdjunto(ctx, "escolar-1", "escolar", reserva.ID, adjunto.ID); !errors.Is(err, ErrAdjuntoRechazado) {
		t.Fatalf("no se descarga un rechazado: %v", err)
	}
}

func TestLimiteDeCincoAdjuntos(t *testing.T) {
	servicio, _ := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	ctx := context.Background()
	reserva, _, _ := servicio.Reservar(ctx, "escolar-1", datosValidos())
	for i := 0; i < MaxAdjuntos; i++ {
		if _, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "practica.pdf", pdfDePrueba()); err != nil {
			t.Fatalf("adjunto %d: %v", i, err)
		}
	}
	if _, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "practica.pdf", pdfDePrueba()); !errors.Is(err, ErrAdjuntosLlenos) {
		t.Fatalf("el sexto adjunto debe rechazarse: %v", err)
	}
}

func TestAdjuntoInvalidoPorTipo(t *testing.T) {
	servicio, _ := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	ctx := context.Background()
	reserva, _, _ := servicio.Reservar(ctx, "escolar-1", datosValidos())
	if _, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "pagina.html", []byte("<html><body>hola</body></html>")); !errors.Is(err, ErrAdjuntoInvalido) {
		t.Fatalf("tipo no permitido: %v", err)
	}
	if _, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "..\\practica.pdf", pdfDePrueba()); err == nil {
		t.Fatal("un nombre con ruta debe rechazarse")
	}
}

func TestPermisosSobreAdjuntos(t *testing.T) {
	servicio, _ := servicioDePrueba(slotEn("slot-1", ahora.Add(72*time.Hour)))
	ctx := context.Background()
	reserva, _, _ := servicio.Reservar(ctx, "escolar-1", datosValidos())
	adjunto, err := servicio.Adjuntar(ctx, "escolar-1", reserva.ID, "practica.pdf", pdfDePrueba())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := servicio.Adjuntar(ctx, "escolar-2", reserva.ID, "practica.pdf", pdfDePrueba()); !errors.Is(err, ErrNoAutorizado) {
		t.Fatalf("otro escolar no sube adjuntos: %v", err)
	}
	if _, _, err := servicio.AbrirAdjunto(ctx, "escolar-2", "escolar", reserva.ID, adjunto.ID); !errors.Is(err, ErrNoAutorizado) {
		t.Fatalf("otro escolar no descarga: %v", err)
	}
	if _, _, err := servicio.AbrirAdjunto(ctx, TutorSinteticoID, "tutor", reserva.ID, adjunto.ID); err != nil {
		t.Fatalf("el tutor de la reserva sí descarga: %v", err)
	}
	if _, _, err := servicio.AbrirAdjunto(ctx, "coord-1", "coordinador", reserva.ID, adjunto.ID); err != nil {
		t.Fatalf("coordinación supervisa y descarga: %v", err)
	}
	if _, err := servicio.AdjuntosDe(ctx, "escolar-2", "escolar", reserva.ID); !errors.Is(err, ErrNoAutorizado) {
		t.Fatalf("otro escolar no lista los adjuntos: %v", err)
	}
}

func TestTipoDetectadoYHuella(t *testing.T) {
	if TipoDetectado(pdfDePrueba()) != "application/pdf" {
		t.Fatalf("pdf detectado: %s", TipoDetectado(pdfDePrueba()))
	}
	if HuellaAdjunto([]byte("a")) == HuellaAdjunto([]byte("b")) {
		t.Fatal("huellas distintas para contenidos distintos")
	}
	if _, ok := ExtensionPermitida("text/html"); ok {
		t.Fatal("html no debe estar permitido")
	}
}
