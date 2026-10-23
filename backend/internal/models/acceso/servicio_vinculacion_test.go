package acceso

import (
	"context"
	"errors"
	"testing"
	"time"
)

func crearCuenta(t *testing.T, s *Servicio, email, nombre string, rol Rol) *Usuario {
	t.Helper()
	usuario, err := s.CrearAdministrado(context.Background(), DatosRegistro{
		Email: email, NombreUso: nombre, Clave: claveOK, Rol: rol,
	})
	if err != nil {
		t.Fatal(err)
	}
	return usuario
}

func capturarTokenVinculacion(bus *busDePrueba) *string {
	token := new(string)
	bus.Registrar(VinculacionSolicitada, nuevoObservadorDePrueba("captura-vinculacion", func(e Event) {
		*token, _ = e.Payload["token"].(string)
	}))
	return token
}

func TestVinculacionFlujoCompleto(t *testing.T) {
	s, bus, _, _ := nuevo(t)
	ctx := context.Background()
	apoderado := crearCuenta(t, s, "apoderado@example.com", "Apoderado", RolApoderado)
	escolar := crearCuenta(t, s, "escolar@example.com", "Escolar", RolEscolar)
	token := capturarTokenVinculacion(bus)

	creada, err := s.SolicitarVinculacion(ctx, apoderado, " Escolar@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if creada.Estado != EstadoVinculacionPendiente || *token == "" {
		t.Fatalf("solicitud pendiente con token: %+v", creada)
	}

	consultada, err := s.ConsultarVinculacion(ctx, escolar, *token)
	if err != nil || consultada.ApoderadoEmail != apoderado.Email {
		t.Fatalf("consulta: %+v (%v)", consultada, err)
	}

	aceptada, err := s.AceptarVinculacion(ctx, escolar, *token)
	if err != nil {
		t.Fatal(err)
	}
	if aceptada.Estado != EstadoVinculacionAceptada || aceptada.AceptadaEn == nil {
		t.Fatalf("aceptada: %+v", aceptada)
	}
	if aceptada.Autorizacion == nil || !aceptada.Autorizacion.Vigente() {
		t.Fatalf("al aceptar se otorga la autorización inicial: %+v", aceptada.Autorizacion)
	}
	if autorizado, err := s.AutorizacionVigente(ctx, escolar.ID); err != nil || !autorizado {
		t.Fatalf("autorización inicial vigente: %v (%v)", autorizado, err)
	}
	if _, err := s.AceptarVinculacion(ctx, escolar, *token); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("el enlace es de un solo uso: %v", err)
	}

	revocada, err := s.CambiarAutorizacion(ctx, apoderado, escolar.ID, false)
	if err != nil || revocada.Vigente() || revocada.Estado != AutorizacionRevocada {
		t.Fatalf("revocar: %+v (%v)", revocada, err)
	}
	if autorizado, err := s.AutorizacionVigente(ctx, escolar.ID); err != nil || autorizado {
		t.Fatalf("tras revocar no puede reservar: %v (%v)", autorizado, err)
	}
	otorgada, err := s.CambiarAutorizacion(ctx, apoderado, escolar.ID, true)
	if err != nil || !otorgada.Vigente() {
		t.Fatalf("volver a autorizar: %+v (%v)", otorgada, err)
	}
	if autorizado, err := s.AutorizacionVigente(ctx, escolar.ID); err != nil || !autorizado {
		t.Fatalf("tras autorizar puede reservar: %v (%v)", autorizado, err)
	}

	actual, err := s.VinculacionDeEscolar(ctx, escolar)
	if err != nil || actual == nil || actual.ApoderadoID != apoderado.ID {
		t.Fatalf("vínculo actual: %+v (%v)", actual, err)
	}

	otro := crearCuenta(t, s, "otro@example.com", "Otro", RolApoderado)
	if _, err := s.SolicitarVinculacion(ctx, otro, escolar.Email); !errors.Is(err, ErrYaVinculado) {
		t.Fatalf("un escolar solo admite un apoderado: %v", err)
	}
	if _, err := s.CambiarAutorizacion(ctx, otro, escolar.ID, false); !errors.Is(err, ErrVinculacionAjena) {
		t.Fatalf("otro apoderado no cambia la autorización: %v", err)
	}

	vinculaciones, err := s.VinculacionesDeApoderado(ctx, apoderado)
	if err != nil || len(vinculaciones) != 1 || vinculaciones[0].EscolarEmail != escolar.Email {
		t.Fatalf("listado del apoderado: %+v (%v)", vinculaciones, err)
	}

	historial, err := s.HistorialAutorizaciones(ctx, apoderado, escolar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(historial) != 3 {
		t.Fatalf("debía registrar aceptación, revocación y nueva autorización: %+v", historial)
	}
	esperado := []EstadoAutorizacion{AutorizacionOtorgada, AutorizacionRevocada, AutorizacionOtorgada}
	for i, estado := range esperado {
		if historial[i].Estado != estado {
			t.Fatalf("historial[%d]=%s, se esperaba %s", i, historial[i].Estado, estado)
		}
		if historial[i].CreadaEn.IsZero() {
			t.Fatalf("el cambio %d debe llevar fecha", i)
		}
	}
	if _, err := s.HistorialAutorizaciones(ctx, otro, escolar.ID); !errors.Is(err, ErrVinculacionAjena) {
		t.Fatalf("otro apoderado no ve el historial: %v", err)
	}
	if _, err := s.HistorialAutorizaciones(ctx, escolar, escolar.ID); !errors.Is(err, ErrNoEsApoderado) {
		t.Fatalf("solo el apoderado ve el historial: %v", err)
	}
}

func TestVinculacionSolicitudPendienteYUnApoderadoConVariosEscolares(t *testing.T) {
	s, bus, _, _ := nuevo(t)
	ctx := context.Background()
	apoderado := crearCuenta(t, s, "apoderado@example.com", "Apoderado", RolApoderado)
	escolarUno := crearCuenta(t, s, "uno@example.com", "Uno", RolEscolar)
	escolarDos := crearCuenta(t, s, "dos@example.com", "Dos", RolEscolar)
	token := capturarTokenVinculacion(bus)

	if _, err := s.SolicitarVinculacion(ctx, apoderado, escolarUno.Email); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, escolarUno.Email); !errors.Is(err, ErrSolicitudPendiente) {
		t.Fatalf("no se duplica una solicitud pendiente: %v", err)
	}
	otro := crearCuenta(t, s, "otro@example.com", "Otro", RolApoderado)
	if _, err := s.SolicitarVinculacion(ctx, otro, escolarUno.Email); !errors.Is(err, ErrSolicitudPendiente) {
		t.Fatalf("otro apoderado no desplaza la solicitud pendiente: %v", err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, escolarDos.Email); err != nil {
		t.Fatalf("un apoderado puede solicitar varios escolares: %v", err)
	}
	if _, err := s.AceptarVinculacion(ctx, escolarDos, *token); err != nil {
		t.Fatal(err)
	}
	vinculaciones, err := s.VinculacionesDeApoderado(ctx, apoderado)
	if err != nil || len(vinculaciones) != 2 {
		t.Fatalf("debía tener dos escolares: %+v (%v)", vinculaciones, err)
	}
}

func TestVinculacionVenceYSePuedeVolverASolicitar(t *testing.T) {
	s, bus, ahora, _ := nuevo(t)
	ctx := context.Background()
	apoderado := crearCuenta(t, s, "apoderado@example.com", "Apoderado", RolApoderado)
	escolar := crearCuenta(t, s, "escolar@example.com", "Escolar", RolEscolar)
	token := capturarTokenVinculacion(bus)

	if _, err := s.SolicitarVinculacion(ctx, apoderado, escolar.Email); err != nil {
		t.Fatal(err)
	}
	*ahora = ahora.Add(24*time.Hour + time.Second)
	if _, err := s.ConsultarVinculacion(ctx, escolar, *token); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("consulta vencida: %v", err)
	}
	if _, err := s.AceptarVinculacion(ctx, escolar, *token); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("aceptación vencida: %v", err)
	}
	vinculaciones, err := s.VinculacionesDeApoderado(ctx, apoderado)
	if err != nil || len(vinculaciones) != 0 {
		t.Fatalf("la solicitud vencida no debe listarse: %+v (%v)", vinculaciones, err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, escolar.Email); err != nil {
		t.Fatalf("tras vencer se puede volver a solicitar: %v", err)
	}
}

func TestVinculacionValidaRolesYDestino(t *testing.T) {
	s, _, _, _ := nuevo(t)
	ctx := context.Background()
	apoderado := crearCuenta(t, s, "apoderado@example.com", "Apoderado", RolApoderado)
	escolar := crearCuenta(t, s, "escolar@example.com", "Escolar", RolEscolar)
	tutor := crearCuenta(t, s, "tutor@uni.edu.pe", "Tutor", RolTutor)

	if _, err := s.SolicitarVinculacion(ctx, escolar, apoderado.Email); !errors.Is(err, ErrNoEsApoderado) {
		t.Fatalf("solo apoderado solicita: %v", err)
	}
	if _, err := s.AceptarVinculacion(ctx, apoderado, "token"); !errors.Is(err, ErrNoEsEscolar) {
		t.Fatalf("solo escolar acepta: %v", err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, "nadie@example.com"); !errors.Is(err, ErrEscolarNoEncontrado) {
		t.Fatalf("escolar inexistente: %v", err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, tutor.Email); !errors.Is(err, ErrEscolarNoValido) {
		t.Fatalf("un tutor no es escolar: %v", err)
	}
	if _, err := s.SolicitarVinculacion(ctx, apoderado, ""); !errors.As(err, new(ErrValidacion)) {
		t.Fatalf("correo vacío: %v", err)
	}
	if _, err := s.CambiarAutorizacion(ctx, apoderado, escolar.ID, false); !errors.Is(err, ErrSinApoderado) {
		t.Fatalf("sin vínculo no hay autorización que cambiar: %v", err)
	}
	if _, err := s.CambiarAutorizacion(ctx, escolar, escolar.ID, false); !errors.Is(err, ErrNoEsApoderado) {
		t.Fatalf("solo el apoderado cambia la autorización: %v", err)
	}
	if _, err := s.CambiarAutorizacion(ctx, apoderado, "", false); !errors.As(err, new(ErrValidacion)) {
		t.Fatalf("escolar vacío: %v", err)
	}
}
