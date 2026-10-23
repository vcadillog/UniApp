package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"uniapp/internal/adapters"
	"uniapp/internal/config"
	ctrl "uniapp/internal/controllers/acceso"
	ctrlOferta "uniapp/internal/controllers/oferta"
	ctrlReserva "uniapp/internal/controllers/reserva"
	ctrlSesion "uniapp/internal/controllers/sesion"
	ctrlTutor "uniapp/internal/controllers/tutor"
	infra "uniapp/internal/infra"
	modelo "uniapp/internal/models/acceso"
	modeloReserva "uniapp/internal/models/reserva"
	modeloSesion "uniapp/internal/models/sesion"
	modeloTutor "uniapp/internal/models/tutor"
	"uniapp/migrations"
)

const (
	limiteDeEnvio     = 10 * time.Second
	intentosDeEnvio   = 3
	esperaEntreEnvios = 2 * time.Second
	intervaloLimpieza = time.Minute
)

func main() {
	registro := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	configuracion, err := config.Cargar()
	if err != nil {
		registro.Fatal(err)
	}
	registro.Printf("frontend de enlaces: %s (APP_ENV=%s; orígenes permitidos: %v)", configuracion.OrigenFrontend, configuracion.Entorno, configuracion.OrigenesFrontend)

	ctx, detener := context.WithCancel(context.Background())
	defer detener()

	baseDatos, err := migrations.Abrir(configuracion.DatabaseURL)
	if err != nil {
		registro.Fatalf("base de datos: %v", err)
	}

	if err := migrations.Migrar(ctx, baseDatos, migrations.FS); err != nil {
		registro.Fatalf("migraciones: %v", err)
	}

	servicio := construirServicio(ctx, registro, configuracion, baseDatos)
	go limpiarRegistrosVencidosPeriodicamente(ctx, registro, servicio)

	if configuracion.SembrarSintetico {
		repoSesion := modeloSesion.RepositorioPostgres{DB: baseDatos}

		if err := modeloSesion.SembrarSintetico(
			ctx,
			baseDatos,
			repoSesion,
			primerDominio(configuracion.DominiosTutor),
		); err != nil {
			registro.Fatalf("semilla I05: %v", err)
		}

		registro.Println("sesion sintetica I05 lista")
	}

	controlador := ctrl.Nuevo(servicio, configuracion.CookieSegura)
	servicioTutor := modeloTutor.NuevoServicio(
		modeloTutor.RepositorioPostgres{DB: baseDatos},
		adapters.AlmacenDocumentosFS{Directorio: configuracion.DirectorioDocumentos},
	)
	tutores := ctrlTutor.Nuevo(servicioTutor, controlador.Autenticar()).Rutas()

	// Servicio del flujo D: reservas y adjuntos, con la autorización vigente del flujo A.
	reservas := construirServicioReserva(baseDatos, configuracion, servicio)

	// Extensión del flujo E: sesiones y asistencias.
	manejador := construirManejadorConSesion(
		controlador,
		tutores,
		reservas,
		registro,
		configuracion,
		baseDatos,
	)

	servidor := nuevoServidor(configuracion.Puerto, manejador)
	servirHastaSenalDeParada(registro, servidor, ctx)
}

func construirServicio(ctx context.Context, registro *log.Logger, configuracion config.Config, baseDatos *sql.DB) *modelo.Servicio {
	mailer := construirMailer(configuracion)
	bus := construirBus(registro, mailer, configuracion)
	hasher := infra.NuevoHasher()
	fabrica := modelo.NuevaFabrica(hasher, configuracion.DominiosTutor)

	servicio := modelo.NuevoServicio(modelo.Dependencias{
		Repositorio: modelo.RepositorioPostgres{DB: baseDatos},
		Fabrica:     fabrica,
		Hasher:      hasher,
		Bus:         bus,
	})

	if liberados, err := servicio.LimpiarRegistrosVencidos(ctx); err != nil {
		registro.Printf("limpieza de registros sin confirmar: %v", err)
	} else if liberados > 0 {
		registro.Printf("registros sin confirmar vencidos liberados: %d", liberados)
	}

	if configuracion.SembrarSintetico {
		if err := modelo.Sembrar(ctx, servicio, modelo.OpcionesSiembra{DominioTutor: configuracion.DominiosTutor, Clave: configuracion.ClaveSemilla}); err != nil {
			registro.Fatalf("semilla: %v", err)
		}
		if err := modelo.SembrarVinculacion(ctx, servicio, modelo.EmailApoderadoSintetico, modelo.EmailEscolarSintetico); err != nil {
			registro.Fatalf("semilla de vinculación: %v", err)
		}

		registro.Println("cuentas sintéticas de prueba listas")
	}

	return servicio
}

// Servicio del flujo D: reserva transaccional y adjuntos en cuarentena (I09),
// con la autorización vigente del flujo A inyectada como puerto.
func construirServicioReserva(baseDatos *sql.DB, configuracion config.Config, autorizacion modeloReserva.PuertoAutorizacion) *modeloReserva.Servicio {
	var analizador modeloReserva.Analizador = modeloReserva.AnalizadorBasico{}
	if strings.EqualFold(configuracion.AnalisisAdjuntos, "ninguno") {
		// Sin análisis disponible la reserva sigue funcionando: el adjunto queda en cuarentena.
		analizador = modeloReserva.AnalizadorNoDisponible{}
	}
	return modeloReserva.NuevoServicio(modeloReserva.Dependencias{
		Repositorio:  modeloReserva.RepositorioPostgres{DB: baseDatos},
		Oferta:       modeloReserva.NuevaOfertaSintetica(time.Now().UTC()),
		Almacen:      adapters.AlmacenAdjuntosFS{Directorio: configuracion.DirectorioAdjuntos},
		Analizador:   analizador,
		Limites:      modeloReserva.LimitesPorDefecto,
		Autorizacion: autorizacion,
	})
}

func construirMailer(configuracion config.Config) modelo.Mailer {
	var mailer modelo.Mailer = infra.LogMailer{MostrarCuerpo: !configuracion.CookieSegura}
	if configuracion.SMTP.Host != "" {
		mailer = infra.SMTPMailer{
			Host: configuracion.SMTP.Host, Puerto: configuracion.SMTP.Puerto,
			Usuario: configuracion.SMTP.Usuario, Clave: configuracion.SMTP.Clave,
			Remitente: configuracion.SMTP.Remitente,
		}
	}
	return infra.NuevoReintentoMailer(infra.NuevoTiempoLimiteMailer(mailer, limiteDeEnvio), intentosDeEnvio, esperaEntreEnvios)
}

func construirBus(registro *log.Logger, mailer modelo.Mailer, configuracion config.Config) *infra.Bus {
	bus := infra.NuevoBus()
	auditoria := infra.NuevaAuditoria(registro)
	for _, nombre := range []string{modelo.UsuarioRegistrado, modelo.IngresoExitoso, modelo.IngresoFallido, modelo.ClaveRestablecida, modelo.RecuperacionSolicitada, modelo.ConfirmacionSolicitada, modelo.CorreoConfirmado, modelo.VinculacionSolicitada, modelo.VinculacionAceptada} {
		bus.Registrar(nombre, auditoria)
	}
	bus.Registrar(modelo.RecuperacionSolicitada, infra.NuevoCorreoRecuperacion(mailer, configuracion.OrigenFrontend))
	bus.Registrar(modelo.ConfirmacionSolicitada, infra.NuevoCorreoConfirmacion(mailer, configuracion.OrigenFrontend))
	bus.Registrar(modelo.VinculacionSolicitada, infra.NuevoCorreoVinculacion(mailer, configuracion.OrigenFrontend))
	return bus
}

// Manejador principal: conserva las rutas de acceso y reservas.
func construirManejador(
	controlador *ctrl.Controlador,
	tutores http.Handler,
	reservas *modeloReserva.Servicio,
	registro *log.Logger,
	configuracion config.Config,
	baseDatos *sql.DB,
) http.Handler {
	raiz := http.NewServeMux()

	raiz.Handle("/api/tutor/", tutores)
	raiz.Handle("/api/oferta/", ctrlOferta.Rutas(baseDatos, controlador.Autenticar()))

	// Ambos patrones son necesarios para el flujo de reservas.
	manejadorReserva := ctrlReserva.Nuevo(
		reservas,
		controlador.Autenticar(),
	).Rutas()

	raiz.Handle("/api/reserva", manejadorReserva)
	raiz.Handle("/api/reserva/", manejadorReserva)

	raiz.Handle("/", controlador.Rutas())

	return ctrl.Encadenar(
		raiz,
		ctrl.Recuperar(registro),
		ctrl.Registro(registro),
		ctrl.CabecerasSeguras(),
		ctrl.OrigenPermitido(configuracion.OrigenesFrontend...),
	)
}

// Extensión I05: agrega sesiones sin sustituir el flujo D.
func construirManejadorConSesion(
	controlador *ctrl.Controlador,
	tutores http.Handler,
	reservas *modeloReserva.Servicio,
	registro *log.Logger,
	configuracion config.Config,
	baseDatos *sql.DB,
) http.Handler {
	repositorioSesion := modeloSesion.RepositorioPostgres{
		DB: baseDatos,
	}

	rutasSesion := ctrlSesion.Rutas(
		repositorioSesion,
		controlador.Autenticar(),
		configuracion.OrigenFrontend,
	)

	raiz := http.NewServeMux()

	raiz.Handle(
		"/api/sesion/",
		ctrl.Encadenar(
			rutasSesion,
			ctrl.Recuperar(registro),
			ctrl.Registro(registro),
			ctrl.CabecerasSeguras(),
			ctrl.OrigenPermitido(configuracion.OrigenesFrontend...),
		),
	)

	raiz.Handle(
		"/",
		construirManejador(
			controlador,
			tutores,
			reservas,
			registro,
			configuracion,
			baseDatos,
		),
	)

	return raiz
}

func nuevoServidor(puerto string, manejador http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + puerto,
		Handler:           manejador,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
}

func limpiarRegistrosVencidosPeriodicamente(ctx context.Context, registro *log.Logger, servicio *modelo.Servicio) {
	ticker := time.NewTicker(intervaloLimpieza)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if liberados, err := servicio.LimpiarRegistrosVencidos(ctx); err != nil {
				registro.Printf("limpieza de registros sin confirmar: %v", err)
			} else if liberados > 0 {
				registro.Printf("registros sin confirmar vencidos liberados: %d", liberados)
			}
		case <-ctx.Done():
			return
		}
	}
}

func servirHastaSenalDeParada(registro *log.Logger, servidor *http.Server, ctx context.Context) {
	go func() {
		registro.Printf("API escuchando en %s", servidor.Addr)
		if err := servidor.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			registro.Fatal(err)
		}
	}()

	senalDeParada := make(chan os.Signal, 1)
	signal.Notify(senalDeParada, os.Interrupt, syscall.SIGTERM)
	<-senalDeParada

	contextoDeCierre, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	_ = servidor.Shutdown(contextoDeCierre)
}

// primerDominio devuelve el primer dominio no vacío de la lista separada por comas.
// I05 solo siembra una sesión y necesita el mismo dominio que usa el seed de acceso.
func primerDominio(texto string) string {
	for _, parte := range strings.Split(texto, ",") {
		if dominio := strings.TrimSpace(parte); dominio != "" {
			return dominio
		}
	}
	return ""
}
