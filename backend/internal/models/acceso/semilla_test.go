package acceso

import (
	"context"
	"testing"
)

func TestSembrarVinculacionSinteticaEsIdempotente(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := nuevo(t)
	if err := Sembrar(ctx, s, OpcionesSiembra{DominioTutor: "uni.edu.pe", Clave: claveOK}); err != nil {
		t.Fatal(err)
	}

	if err := SembrarVinculacion(ctx, s, EmailApoderadoSintetico, EmailEscolarSintetico); err != nil {
		t.Fatal(err)
	}
	apoderado, err := s.repositorio.UsuarioPorEmail(ctx, EmailApoderadoSintetico)
	if err != nil {
		t.Fatal(err)
	}
	escolar, err := s.repositorio.UsuarioPorEmail(ctx, EmailEscolarSintetico)
	if err != nil {
		t.Fatal(err)
	}

	vinculacion, err := s.repositorio.VinculacionAceptadaDeEscolar(ctx, escolar.ID)
	if err != nil || vinculacion.ApoderadoID != apoderado.ID {
		t.Fatalf("vínculo sintético: %+v (%v)", vinculacion, err)
	}
	historial, err := s.HistorialAutorizaciones(ctx, apoderado, escolar.ID)
	if err != nil || len(historial) != 1 || !historial[0].Vigente() {
		t.Fatalf("autorización inicial: %+v (%v)", historial, err)
	}

	// Volver a arrancar la semilla no debe duplicar el vínculo ni el historial.
	if err := SembrarVinculacion(ctx, s, EmailApoderadoSintetico, EmailEscolarSintetico); err != nil {
		t.Fatal(err)
	}
	historial, err = s.HistorialAutorizaciones(ctx, apoderado, escolar.ID)
	if err != nil || len(historial) != 1 {
		t.Fatalf("semilla idempotente: %+v (%v)", historial, err)
	}
}
