package acceso_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	infra "uniapp/internal/infra"
	modelo "uniapp/internal/models/acceso"
	"uniapp/migrations"
)

func TestRepositorioPostgresFlujoCompleto(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	d, err := migrations.Abrir(url)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := migrations.Migrar(ctx, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Migrar(ctx, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Exec(`TRUNCATE usuarios CASCADE`)

	h := infra.PBKDF2Hasher{Iteraciones: 1000}
	repo := modelo.RepositorioPostgres{DB: d}
	bus := infra.NuevoBus()
	var tokenConfirmacion string
	bus.Registrar(modelo.ConfirmacionSolicitada, infra.NuevoObservadorFunc("captura-confirmacion", func(e modelo.Event) {
		tokenConfirmacion, _ = e.Payload["token"].(string)
	}))
	s := modelo.NuevoServicio(modelo.Dependencias{
		Repositorio: repo,
		Fabrica:     modelo.NuevaFabrica(h, "uni.edu.pe"),
		Hasher:      h,
		Bus:         bus,
	})
	registrado, err := s.Registrar(ctx, modelo.DatosRegistro{Email: "pg@example.com", NombreUso: "Pg", Clave: "clave-larga-ok", Rol: modelo.RolEscolar})
	if err != nil {
		t.Fatal(err)
	}
	if registrado.EmailConfirmado || tokenConfirmacion == "" {
		t.Fatal("el registro público debe nacer sin confirmar y emitir el enlace")
	}
	if _, err := s.Registrar(ctx, modelo.DatosRegistro{Email: "PG@example.com", NombreUso: "Pg", Clave: "clave-larga-ok", Rol: modelo.RolEscolar}); !errors.Is(err, modelo.ErrEmailEnUso) {
		t.Fatalf("unicidad: %v", err)
	}
	if _, _, err := s.Ingresar(ctx, "pg@example.com", "clave-larga-ok"); !errors.Is(err, modelo.ErrCredenciales) {
		t.Fatalf("sin confirmar debe ser invisible: %v", err)
	}
	if err := s.ConfirmarCorreo(ctx, tokenConfirmacion); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmarCorreo(ctx, tokenConfirmacion); !errors.Is(err, modelo.ErrTokenInvalido) {
		t.Fatal("el enlace de confirmación es de un solo uso")
	}
	confirmado, err := repo.UsuarioPorEmail(ctx, "pg@example.com")
	if err != nil || !confirmado.EmailConfirmado {
		t.Fatalf("correo_confirmado debe persistir: %v", err)
	}
	tok, u, err := s.Ingresar(ctx, "pg@example.com", "clave-larga-ok")
	if err != nil || u.Rol != modelo.RolEscolar {
		t.Fatal(err)
	}
	if got, err := s.UsuarioDeSesion(ctx, tok); err != nil || got.UltimoIngreso == nil {
		t.Fatalf("sesión: %v", err)
	}

	var tokenReset string
	bus.Registrar(modelo.RecuperacionSolicitada, infra.NuevoObservadorFunc("captura-token-reset", func(e modelo.Event) {
		tokenReset, _ = e.Payload["token"].(string)
	}))
	_ = s.SolicitarRecuperacion(ctx, "pg@example.com")
	if err := s.RestablecerClave(ctx, tokenReset, "nueva-clave-larga"); err != nil {
		t.Fatal(err)
	}
	if err := s.RestablecerClave(ctx, tokenReset, "nueva-clave-larga"); !errors.Is(err, modelo.ErrTokenInvalido) {
		t.Fatal("un solo uso")
	}
	if _, err := s.UsuarioDeSesion(ctx, tok); err == nil {
		t.Fatal("sesiones cerradas tras restablecer")
	}

	if _, err := s.Registrar(ctx, modelo.DatosRegistro{Email: "vencido@example.com", NombreUso: "Vencido", Clave: "clave-larga-ok", Rol: modelo.RolEscolar}); err != nil {
		t.Fatal(err)
	}
	liberados, err := repo.EliminarUsuariosSinConfirmar(ctx, time.Now().Add(31*time.Minute))
	if err != nil || liberados != 1 {
		t.Fatalf("debía liberarse el registro vencido: %d (%v)", liberados, err)
	}
	if _, err := repo.UsuarioPorEmail(ctx, "vencido@example.com"); !errors.Is(err, modelo.ErrNoEncontrado) {
		t.Fatalf("el correo debe quedar libre: %v", err)
	}
	if confirmado, err := repo.UsuarioPorEmail(ctx, "pg@example.com"); err != nil || !confirmado.EmailConfirmado {
		t.Fatalf("no debe borrar cuentas confirmadas: %v", err)
	}
}
func TestRepositorioPostgresVinculacion(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	d, err := migrations.Abrir(url)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := migrations.Migrar(ctx, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Exec(`TRUNCATE usuarios CASCADE`)

	h := infra.PBKDF2Hasher{Iteraciones: 1000}
	bus := infra.NuevoBus()
	var tokenVinculacion string
	bus.Registrar(modelo.VinculacionSolicitada, infra.NuevoObservadorFunc("captura-vinculacion-pg", func(e modelo.Event) {
		tokenVinculacion, _ = e.Payload["token"].(string)
	}))
	s := modelo.NuevoServicio(modelo.Dependencias{
		Repositorio: modelo.RepositorioPostgres{DB: d},
		Fabrica:     modelo.NuevaFabrica(h, "uni.edu.pe"),
		Hasher:      h,
		Bus:         bus,
	})
	apoderado, err := s.CrearAdministrado(ctx, modelo.DatosRegistro{
		Email: "apoderado.pg@example.com", NombreUso: "Apoderado PG", Clave: "clave-larga-ok", Rol: modelo.RolApoderado,
	})
	if err != nil {
		t.Fatal(err)
	}
	escolar, err := s.CrearAdministrado(ctx, modelo.DatosRegistro{
		Email: "escolar.pg@example.com", NombreUso: "Escolar PG", Clave: "clave-larga-ok", Rol: modelo.RolEscolar,
	})
	if err != nil {
		t.Fatal(err)
	}

	creada, err := s.SolicitarVinculacion(ctx, apoderado, escolar.Email)
	if err != nil {
		t.Fatal(err)
	}
	if creada.Estado != modelo.EstadoVinculacionPendiente || tokenVinculacion == "" {
		t.Fatalf("solicitud: %+v", creada)
	}
	consultada, err := s.ConsultarVinculacion(ctx, escolar, tokenVinculacion)
	if err != nil || consultada.ApoderadoEmail != apoderado.Email {
		t.Fatalf("consulta: %+v (%v)", consultada, err)
	}
	aceptada, err := s.AceptarVinculacion(ctx, escolar, tokenVinculacion)
	if err != nil || aceptada.Estado != modelo.EstadoVinculacionAceptada || aceptada.AceptadaEn == nil {
		t.Fatalf("aceptar: %+v (%v)", aceptada, err)
	}
	if aceptada.Autorizacion == nil || !aceptada.Autorizacion.Vigente() {
		t.Fatalf("autorización inicial: %+v", aceptada.Autorizacion)
	}
	if autorizado, err := s.AutorizacionVigente(ctx, escolar.ID); err != nil || !autorizado {
		t.Fatalf("autorización vigente: %v (%v)", autorizado, err)
	}
	if _, err := s.CambiarAutorizacion(ctx, apoderado, escolar.ID, false); err != nil {
		t.Fatal(err)
	}
	if autorizado, err := s.AutorizacionVigente(ctx, escolar.ID); err != nil || autorizado {
		t.Fatalf("revocación persistida: %v (%v)", autorizado, err)
	}
	if _, err := s.CambiarAutorizacion(ctx, apoderado, escolar.ID, true); err != nil {
		t.Fatal(err)
	}
	var cambios int
	if err := d.QueryRowContext(ctx,
		`SELECT count(*) FROM autorizaciones_apoderado WHERE escolar_id=$1 AND apoderado_id=$2`,
		escolar.ID, apoderado.ID).Scan(&cambios); err != nil || cambios != 3 {
		t.Fatalf("historial de cambios: %d (%v)", cambios, err)
	}
	historial, err := s.HistorialAutorizaciones(ctx, apoderado, escolar.ID)
	if err != nil || len(historial) != 3 {
		t.Fatalf("historial del vínculo: %+v (%v)", historial, err)
	}
	esperado := []modelo.EstadoAutorizacion{
		modelo.AutorizacionOtorgada, modelo.AutorizacionRevocada, modelo.AutorizacionOtorgada,
	}
	for i, estado := range esperado {
		if historial[i].Estado != estado {
			t.Fatalf("historial[%d]=%s, se esperaba %s", i, historial[i].Estado, estado)
		}
		if historial[i].CreadaEn.IsZero() {
			t.Fatalf("el cambio %d debe llevar fecha", i)
		}
	}
	if _, err := s.AceptarVinculacion(ctx, escolar, tokenVinculacion); !errors.Is(err, modelo.ErrTokenInvalido) {
		t.Fatalf("un solo uso: %v", err)
	}

	actual, err := s.VinculacionDeEscolar(ctx, escolar)
	if err != nil || actual == nil || actual.ApoderadoID != apoderado.ID {
		t.Fatalf("vínculo: %+v (%v)", actual, err)
	}
	otro, err := s.CrearAdministrado(ctx, modelo.DatosRegistro{
		Email: "otro.pg@example.com", NombreUso: "Otro PG", Clave: "clave-larga-ok", Rol: modelo.RolApoderado,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.HistorialAutorizaciones(ctx, otro, escolar.ID); !errors.Is(err, modelo.ErrVinculacionAjena) {
		t.Fatalf("otro apoderado no ve el historial: %v", err)
	}
	if _, err := s.SolicitarVinculacion(ctx, otro, escolar.Email); !errors.Is(err, modelo.ErrYaVinculado) {
		t.Fatalf("escolar ya vinculado: %v", err)
	}
	vinculaciones, err := s.VinculacionesDeApoderado(ctx, apoderado)
	if err != nil || len(vinculaciones) != 1 {
		t.Fatalf("listado: %+v (%v)", vinculaciones, err)
	}
}
