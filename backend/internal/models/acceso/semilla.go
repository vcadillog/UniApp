package acceso

import (
	"context"
	"errors"
	"fmt"
)

const (
	EmailApoderadoSintetico = "apoderado.prueba@example.test"
	EmailEscolarSintetico   = "escolar.prueba@example.test"
)

type OpcionesSiembra struct {
	DominioTutor string
	Clave        string
}

func Sembrar(ctx context.Context, servicio *Servicio, opciones OpcionesSiembra) error {
	correoTutor := "tutor.prueba@example.test"
	if dominios := dominiosDe(opciones.DominioTutor); len(dominios) > 0 {
		correoTutor = "tutor.prueba@" + dominios[0]
	}
	datos := []DatosRegistro{
		{Email: EmailEscolarSintetico, NombreUso: "Escolar Prueba", Clave: opciones.Clave, Rol: RolEscolar},
		{Email: EmailApoderadoSintetico, NombreUso: "Apoderado Prueba", Clave: opciones.Clave, Rol: RolApoderado},
		{Email: correoTutor, NombreUso: "Tutor Prueba", Clave: opciones.Clave, Rol: RolTutor},
		{Email: "coord.prueba@example.test", NombreUso: "Coordinación Prueba", Clave: opciones.Clave, Rol: RolCoordinador},
	}
	for _, dato := range datos {
		if _, err := servicio.CrearAdministrado(ctx, dato); err != nil && !errors.Is(err, ErrEmailEnUso) {
			return err
		}
	}
	return nil
}

// SembrarVinculacion crea un vínculo aceptado entre dos cuentas sintéticas,
// junto con su autorización inicial para pedir tutorías. Es idempotente: si el
// escolar ya tiene apoderado, no vuelve a crear nada.
func SembrarVinculacion(ctx context.Context, servicio *Servicio, emailApoderado, emailEscolar string) error {
	apoderado, err := servicio.repositorio.UsuarioPorEmail(ctx, normalizarEmail(emailApoderado))
	if err != nil {
		return fmt.Errorf("buscar apoderado sintético: %w", err)
	}
	escolar, err := servicio.repositorio.UsuarioPorEmail(ctx, normalizarEmail(emailEscolar))
	if err != nil {
		return fmt.Errorf("buscar escolar sintético: %w", err)
	}

	if _, err := servicio.repositorio.VinculacionAceptadaDeEscolar(ctx, escolar.ID); err == nil {
		// Ya hay un vínculo aceptado: no se altera, aunque sea de otro apoderado.
		return nil
	} else if !errors.Is(err, ErrNoEncontrado) {
		return err
	}

	token, tokenHash, err := nuevoToken()
	if err != nil {
		return err
	}
	momento := servicio.ahora()
	if _, err := servicio.repositorio.GuardarSolicitudVinculacion(ctx, Vinculacion{
		ApoderadoID: apoderado.ID,
		EscolarID:   escolar.ID,
		Estado:      EstadoVinculacionPendiente,
		TokenHash:   tokenHash,
		ExpiraEn:    momento.Add(duracionVinculacion),
		CreadaEn:    momento,
	}); err != nil {
		return fmt.Errorf("crear vínculo sintético: %w", err)
	}
	if _, err := servicio.repositorio.ConsumirSolicitudVinculacion(ctx, hashToken(token), escolar.ID, momento); err != nil {
		return fmt.Errorf("aceptar vínculo sintético: %w", err)
	}
	return nil
}
