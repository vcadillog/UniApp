package sesion

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Tablas temporales en una única conexión: nunca escribe ni borra tablas públicas.
func TestI05PostgresPersistenciaAislada(t *testing.T) {
	conexion := os.Getenv("I05_TEST_DATABASE_URL")
	if conexion == "" {
		t.Skip("I05_TEST_DATABASE_URL no definida; requiere PostgreSQL disponible")
	}
	// El aislamiento se aplica también a cualquier reconexión del driver.
	if strings.HasPrefix(conexion, "postgres://") || strings.HasPrefix(conexion, "postgresql://") {
		u, err := url.Parse(conexion)
		if err != nil {
			t.Fatal("URL de PostgreSQL inválida")
		}
		q := u.Query()
		q.Set("search_path", "pg_temp")
		u.RawQuery = q.Encode()
		conexion = u.String()
	} else {
		conexion += " search_path=pg_temp"
	}
	db, err := sql.Open("postgres", conexion)
	if err != nil {
		t.Fatal("no se pudo abrir PostgreSQL")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("PostgreSQL no disponible")
	}
	// Si se pierde la conexión, el search_path impide caer en las tablas públicas.
	if _, err := db.ExecContext(ctx, "SET search_path TO pg_temp"); err != nil {
		t.Fatal("no se pudo aislar el esquema")
	}
	migracion, err := os.ReadFile("../../../migrations/005_sesion.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlTemporal := strings.ReplaceAll(string(migracion), "CREATE TABLE", "CREATE TEMP TABLE")
	if _, err := db.ExecContext(ctx, sqlTemporal); err != nil {
		t.Fatal("no se pudieron crear tablas temporales")
	}
	inicio := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	s, err := NuevaSesion("sesion-test-aislada", "reserva-test-aislada", "tutor-test", "escolar-test", inicio, inicio.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	repo := RepositorioPostgres{DB: db}
	if err := repo.CrearSesion(ctx, s); err != nil {
		t.Fatal("falló crear sesión temporal")
	}
	for _, usuario := range []string{"tutor-test", "escolar-test"} {
		if _, err := repo.DeclararAsistencia(ctx, s.ID(), usuario, inicio); err != nil {
			t.Fatal("falló persistir asistencia temporal")
		}
		if _, err := repo.DeclararAsistencia(ctx, s.ID(), usuario, inicio); !errors.Is(err, ErrAsistenciaDuplicada) {
			t.Fatal("no rechazó duplicado")
		}
	}
	if _, err := repo.DeclararAsistencia(ctx, s.ID(), "ajeno", inicio); !errors.Is(err, ErrNoAutorizado) {
		t.Fatal("no rechazó usuario ajeno")
	}
	// Nueva instancia del repositorio: lectura del almacenamiento, no del objeto original.
	otra := RepositorioPostgres{DB: db}
	guardada, err := otra.SesionPorID(ctx, s.ID())
	if err != nil {
		t.Fatal("falló recargar la sesión temporal")
	}
	if len(guardada.Asistencias()) != 2 {
		t.Fatal("asistencias no persistidas")
	}
	for i, rol := range []Rol{RolEscolar, RolTutor} {
		if guardada.Asistencias()[i].Rol() != rol {
			t.Fatal("rol persistido incorrecto")
		}
	}
}
