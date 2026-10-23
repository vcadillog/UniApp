package acceso

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

type RepositorioPostgres struct{ DB *sql.DB }

var idPorRol = map[Rol]int{RolEscolar: 1, RolApoderado: 2, RolTutor: 3, RolCoordinador: 4}

const consultaUsuario = `SELECT u.id, u.email, u.nombre_uso, u.clave_hash, r.nombre, u.activo, u.correo_confirmado, u.creado_en, u.ultimo_ingreso
	FROM usuarios u JOIN roles r ON r.id = u.rol_id `

func (r RepositorioPostgres) CrearUsuario(ctx context.Context, usuario *Usuario) (*Usuario, error) {
	registrado := *usuario
	err := r.DB.QueryRowContext(ctx,
		`INSERT INTO usuarios (email, nombre_uso, clave_hash, rol_id, correo_confirmado) VALUES ($1,$2,$3,$4,$5) RETURNING id, creado_en`,
		usuario.Email, usuario.NombreUso, usuario.ClaveHash, idPorRol[usuario.Rol], usuario.EmailConfirmado).
		Scan(&registrado.ID, &registrado.CreadoEn)
	var errorPostgres *pq.Error
	if errors.As(err, &errorPostgres) && errorPostgres.Code == "23505" {
		return nil, ErrEmailEnUso
	}
	if err != nil {
		return nil, err
	}
	return &registrado, nil
}

func (r RepositorioPostgres) UsuarioPorEmail(ctx context.Context, email string) (*Usuario, error) {
	return escanearUsuario(r.DB.QueryRowContext(ctx, consultaUsuario+`WHERE u.email = $1`, email))
}

func (r RepositorioPostgres) UsuarioPorID(ctx context.Context, id string) (*Usuario, error) {
	return escanearUsuario(r.DB.QueryRowContext(ctx, consultaUsuario+`WHERE u.id = $1`, id))
}

func (r RepositorioPostgres) ActualizarClave(ctx context.Context, id, claveHash string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE usuarios SET clave_hash=$2 WHERE id=$1`, id, claveHash)
	return err
}

func (r RepositorioPostgres) MarcarCorreoConfirmado(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE usuarios SET correo_confirmado=TRUE WHERE id=$1`, id)
	return err
}

func (r RepositorioPostgres) RegistrarIngreso(ctx context.Context, id string, momento time.Time) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE usuarios SET ultimo_ingreso=$2 WHERE id=$1`, id, momento)
	return err
}

func (r RepositorioPostgres) GuardarSesion(ctx context.Context, sesion Sesion) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO sesiones (token_hash, usuario_id, expira_en) VALUES ($1,$2,$3)`,
		sesion.TokenHash, sesion.UsuarioID, sesion.ExpiraEn)
	return err
}

func (r RepositorioPostgres) SesionPorHash(ctx context.Context, tokenHash []byte) (*Sesion, error) {
	var sesion Sesion
	err := r.DB.QueryRowContext(ctx, `SELECT token_hash, usuario_id, expira_en FROM sesiones WHERE token_hash=$1`, tokenHash).
		Scan(&sesion.TokenHash, &sesion.UsuarioID, &sesion.ExpiraEn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	return &sesion, err
}

func (r RepositorioPostgres) EliminarSesion(ctx context.Context, tokenHash []byte) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM sesiones WHERE token_hash=$1`, tokenHash)
	return err
}

func (r RepositorioPostgres) EliminarSesionesDeUsuario(ctx context.Context, id string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id=$1`, id)
	return err
}

func (r RepositorioPostgres) GuardarTokenRecuperacion(ctx context.Context, token TokenRecuperacion) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO tokens_recuperacion (token_hash, usuario_id, expira_en) VALUES ($1,$2,$3)`,
		token.TokenHash, token.UsuarioID, token.ExpiraEn)
	return err
}

func (r RepositorioPostgres) ConsumirTokenRecuperacion(ctx context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`UPDATE tokens_recuperacion SET usado_en=$2 WHERE token_hash=$1 AND usado_en IS NULL AND expira_en > $2 RETURNING usuario_id`,
		tokenHash, ahora).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenInvalido
	}
	return id, err
}

func (r RepositorioPostgres) UsuarioDeTokenRecuperacion(ctx context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT usuario_id FROM tokens_recuperacion WHERE token_hash=$1 AND usado_en IS NULL AND expira_en > $2`, tokenHash, ahora).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenInvalido
	}
	return id, err
}

func (r RepositorioPostgres) GuardarTokenConfirmacion(ctx context.Context, token TokenConfirmacion) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO tokens_confirmacion (token_hash, usuario_id, expira_en) VALUES ($1,$2,$3)`,
		token.TokenHash, token.UsuarioID, token.ExpiraEn)
	return err
}

func (r RepositorioPostgres) ConsumirTokenConfirmacion(ctx context.Context, tokenHash []byte, ahora time.Time) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`UPDATE tokens_confirmacion SET usado_en=$2 WHERE token_hash=$1 AND usado_en IS NULL AND expira_en > $2 RETURNING usuario_id`,
		tokenHash, ahora).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenInvalido
	}
	return id, err
}

// EliminarUsuariosSinConfirmar libera los correos de registros que nunca
// confirmaron: solo conserva las cuentas sin confirmar con algún enlace vivo.
func (r RepositorioPostgres) EliminarUsuariosSinConfirmar(ctx context.Context, ahora time.Time) (int64, error) {
	resultado, err := r.DB.ExecContext(ctx, `DELETE FROM usuarios u
		WHERE NOT u.correo_confirmado AND NOT EXISTS (
			SELECT 1 FROM tokens_confirmacion t
			WHERE t.usuario_id = u.id AND t.usado_en IS NULL AND t.expira_en > $1
		)`, ahora)
	if err != nil {
		return 0, err
	}
	return resultado.RowsAffected()
}

const consultaVinculacion = `SELECT v.id, v.apoderado_id, v.escolar_id, v.estado, v.token_hash,
	v.expira_en, v.creada_en, v.aceptada_en,
	a.nombre_uso, a.email, e.nombre_uso, e.email,
	auth.id, auth.vinculacion_id, auth.apoderado_id, auth.escolar_id, auth.estado, auth.creada_en
	FROM vinculaciones_apoderado v
	JOIN usuarios a ON a.id = v.apoderado_id
	JOIN usuarios e ON e.id = v.escolar_id
	LEFT JOIN LATERAL (
		SELECT au.id, au.vinculacion_id, au.apoderado_id, au.escolar_id, au.estado, au.creada_en
		FROM autorizaciones_apoderado au
		WHERE au.vinculacion_id = v.id
		ORDER BY au.creada_en DESC, au.id DESC
		LIMIT 1
	) auth ON TRUE `

const consultaAutorizacion = `SELECT a.id, a.vinculacion_id, a.apoderado_id, a.escolar_id, a.estado, a.creada_en
	FROM autorizaciones_apoderado a
	JOIN vinculaciones_apoderado v ON v.id = a.vinculacion_id AND v.estado = 'aceptada' `

func (r RepositorioPostgres) GuardarSolicitudVinculacion(ctx context.Context, v Vinculacion) (*Vinculacion, error) {
	creada := v
	err := r.DB.QueryRowContext(ctx,
		`INSERT INTO vinculaciones_apoderado (apoderado_id, escolar_id, estado, token_hash, expira_en)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, creada_en`,
		v.ApoderadoID, v.EscolarID, v.Estado, v.TokenHash, v.ExpiraEn).
		Scan(&creada.ID, &creada.CreadaEn)
	if err != nil {
		return nil, err
	}
	return &creada, nil
}

func (r RepositorioPostgres) SolicitudPendienteDeEscolar(ctx context.Context, escolarID string, ahora time.Time) (*Vinculacion, error) {
	return escanearVinculacion(r.DB.QueryRowContext(ctx,
		consultaVinculacion+`WHERE v.escolar_id=$1 AND v.estado='pendiente' AND v.expira_en > $2
		ORDER BY v.creada_en DESC LIMIT 1`, escolarID, ahora))
}

func (r RepositorioPostgres) SolicitudVinculacionPorToken(ctx context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error) {
	return escanearVinculacion(r.DB.QueryRowContext(ctx,
		consultaVinculacion+`WHERE v.token_hash=$1 AND v.escolar_id=$2 AND v.estado='pendiente' AND v.expira_en > $3
		ORDER BY v.creada_en DESC LIMIT 1`, tokenHash, escolarID, ahora))
}

func (r RepositorioPostgres) VinculacionAceptadaDeEscolar(ctx context.Context, escolarID string) (*Vinculacion, error) {
	vinculacion, err := escanearVinculacion(r.DB.QueryRowContext(ctx,
		consultaVinculacion+`WHERE v.escolar_id=$1 AND v.estado='aceptada'
		ORDER BY v.aceptada_en DESC LIMIT 1`, escolarID))
	if err != nil {
		// Un identificador con formato inválido no debe convertirse en un 500.
		var errorPostgres *pq.Error
		if errors.As(err, &errorPostgres) && errorPostgres.Code == "22P02" {
			return nil, ErrNoEncontrado
		}
		return nil, err
	}
	return vinculacion, nil
}

func (r RepositorioPostgres) ConsumirSolicitudVinculacion(ctx context.Context, tokenHash []byte, escolarID string, ahora time.Time) (*Vinculacion, error) {
	consulta := `WITH actualizada AS (
		UPDATE vinculaciones_apoderado
		SET estado='aceptada', aceptada_en=$3
		WHERE token_hash=$1 AND escolar_id=$2 AND estado='pendiente' AND expira_en > $3
		RETURNING id, apoderado_id, escolar_id, estado, token_hash, expira_en, creada_en, aceptada_en
	), inicial AS (
		INSERT INTO autorizaciones_apoderado (vinculacion_id, apoderado_id, escolar_id, estado, creada_en)
		SELECT id, apoderado_id, escolar_id, 'autorizado', $3 FROM actualizada
		RETURNING id, vinculacion_id, apoderado_id, escolar_id, estado, creada_en
	)
	SELECT v.id, v.apoderado_id, v.escolar_id, v.estado, v.token_hash,
		v.expira_en, v.creada_en, v.aceptada_en,
		a.nombre_uso, a.email, e.nombre_uso, e.email,
		auth.id, auth.vinculacion_id, auth.apoderado_id, auth.escolar_id, auth.estado, auth.creada_en
	FROM actualizada v
	JOIN usuarios a ON a.id = v.apoderado_id
	JOIN usuarios e ON e.id = v.escolar_id
	JOIN inicial auth ON auth.vinculacion_id = v.id`
	vinculacion, err := escanearVinculacion(r.DB.QueryRowContext(ctx, consulta, tokenHash, escolarID, ahora))
	if err == nil {
		return vinculacion, nil
	}

	var errorPostgres *pq.Error
	if errors.As(err, &errorPostgres) && errorPostgres.Code == "23505" {
		return nil, ErrYaVinculado
	}
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrTokenInvalido
	}
	return nil, err
}

func (r RepositorioPostgres) VinculacionesDeApoderado(ctx context.Context, apoderadoID string, ahora time.Time) ([]Vinculacion, error) {
	filas, err := r.DB.QueryContext(ctx, consultaVinculacion+`
		WHERE v.apoderado_id=$1 AND (v.estado='aceptada' OR v.expira_en > $2)
		ORDER BY CASE WHEN v.estado='aceptada' THEN 0 ELSE 1 END, v.creada_en DESC`, apoderadoID, ahora)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	vinculaciones := make([]Vinculacion, 0)
	for filas.Next() {
		vinculacion, err := escanearVinculacion(filas)
		if err != nil {
			return nil, err
		}
		vinculaciones = append(vinculaciones, *vinculacion)
	}
	return vinculaciones, filas.Err()
}

func (r RepositorioPostgres) AutorizacionVigenteDeEscolar(ctx context.Context, escolarID string) (*Autorizacion, error) {
	return escanearAutorizacion(r.DB.QueryRowContext(ctx,
		consultaAutorizacion+`WHERE a.escolar_id=$1
		ORDER BY a.creada_en DESC, a.id DESC LIMIT 1`, escolarID))
}

// AutorizacionesDeVinculacion devuelve el historial completo del vínculo, del
// cambio más reciente al más antiguo.
func (r RepositorioPostgres) AutorizacionesDeVinculacion(ctx context.Context, vinculacionID string) ([]Autorizacion, error) {
	filas, err := r.DB.QueryContext(ctx,
		consultaAutorizacion+`WHERE a.vinculacion_id=$1
		ORDER BY a.creada_en DESC, a.id DESC`, vinculacionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = filas.Close() }()

	autorizaciones := make([]Autorizacion, 0)
	for filas.Next() {
		var autorizacion Autorizacion
		var estado string
		if err := filas.Scan(
			&autorizacion.ID, &autorizacion.VinculacionID, &autorizacion.ApoderadoID,
			&autorizacion.EscolarID, &estado, &autorizacion.CreadaEn,
		); err != nil {
			return nil, err
		}
		autorizacion.Estado = EstadoAutorizacion(estado)
		autorizaciones = append(autorizaciones, autorizacion)
	}
	return autorizaciones, filas.Err()
}

// CambiarAutorizacion bloquea el vínculo para que dos cambios simultáneos no
// generen historial contradictorio. Si el estado pedido ya es el vigente no
// agrega una fila nueva.
func (r RepositorioPostgres) CambiarAutorizacion(ctx context.Context, vinculacionID, escolarID, apoderadoID string, estado EstadoAutorizacion, ahora time.Time) (*Autorizacion, error) {
	transaccion, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = transaccion.Rollback() }()

	var apoderadoDelVinculo, escolarDelVinculo string
	err = transaccion.QueryRowContext(ctx,
		`SELECT apoderado_id, escolar_id FROM vinculaciones_apoderado
		 WHERE id=$1 AND estado='aceptada' FOR UPDATE`,
		vinculacionID).Scan(&apoderadoDelVinculo, &escolarDelVinculo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, err
	}
	if apoderadoDelVinculo != apoderadoID || escolarDelVinculo != escolarID {
		return nil, ErrNoEncontrado
	}

	actual, err := escanearAutorizacion(transaccion.QueryRowContext(ctx,
		`SELECT id, vinculacion_id, apoderado_id, escolar_id, estado, creada_en
		 FROM autorizaciones_apoderado
		 WHERE vinculacion_id=$1 ORDER BY creada_en DESC, id DESC LIMIT 1`,
		vinculacionID))
	if err != nil && !errors.Is(err, ErrNoEncontrado) {
		return nil, err
	}
	if err == nil && actual.Estado == estado {
		if err := transaccion.Commit(); err != nil {
			return nil, err
		}
		return actual, nil
	}

	cambiada := Autorizacion{
		VinculacionID: vinculacionID,
		ApoderadoID:   apoderadoID,
		EscolarID:     escolarID,
		Estado:        estado,
		CreadaEn:      ahora.UTC(),
	}
	if err := transaccion.QueryRowContext(ctx,
		`INSERT INTO autorizaciones_apoderado (vinculacion_id, apoderado_id, escolar_id, estado, creada_en)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		cambiada.VinculacionID, cambiada.ApoderadoID, cambiada.EscolarID, cambiada.Estado, cambiada.CreadaEn).
		Scan(&cambiada.ID); err != nil {
		return nil, err
	}
	if err := transaccion.Commit(); err != nil {
		return nil, err
	}
	return &cambiada, nil
}

type escaneador interface {
	Scan(destinos ...any) error
}

func escanearVinculacion(fila escaneador) (*Vinculacion, error) {
	var vinculacion Vinculacion
	var estado string
	var aceptadaEn sql.NullTime
	var autorizacionID, autorizacionVinculacionID, autorizacionApoderadoID, autorizacionEscolarID, autorizacionEstado sql.NullString
	var autorizacionCreadaEn sql.NullTime
	if err := fila.Scan(
		&vinculacion.ID, &vinculacion.ApoderadoID, &vinculacion.EscolarID, &estado,
		&vinculacion.TokenHash, &vinculacion.ExpiraEn, &vinculacion.CreadaEn, &aceptadaEn,
		&vinculacion.ApoderadoNombre, &vinculacion.ApoderadoEmail,
		&vinculacion.EscolarNombre, &vinculacion.EscolarEmail,
		&autorizacionID, &autorizacionVinculacionID, &autorizacionApoderadoID, &autorizacionEscolarID,
		&autorizacionEstado, &autorizacionCreadaEn,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncontrado
		}
		return nil, err
	}
	vinculacion.Estado = EstadoVinculacion(estado)
	if aceptadaEn.Valid {
		momento := aceptadaEn.Time
		vinculacion.AceptadaEn = &momento
	}
	if autorizacionID.Valid {
		vinculacion.Autorizacion = &Autorizacion{
			ID:            autorizacionID.String,
			VinculacionID: autorizacionVinculacionID.String,
			ApoderadoID:   autorizacionApoderadoID.String,
			EscolarID:     autorizacionEscolarID.String,
			Estado:        EstadoAutorizacion(autorizacionEstado.String),
			CreadaEn:      autorizacionCreadaEn.Time,
		}
	}
	return &vinculacion, nil
}

func escanearAutorizacion(fila escaneador) (*Autorizacion, error) {
	var autorizacion Autorizacion
	var estado string
	if err := fila.Scan(
		&autorizacion.ID, &autorizacion.VinculacionID, &autorizacion.ApoderadoID,
		&autorizacion.EscolarID, &estado, &autorizacion.CreadaEn,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncontrado
		}
		return nil, err
	}
	autorizacion.Estado = EstadoAutorizacion(estado)
	return &autorizacion, nil
}

func escanearUsuario(fila *sql.Row) (*Usuario, error) {
	var usuario Usuario
	var nombreRol string
	var ultimoIngreso sql.NullTime
	if err := fila.Scan(&usuario.ID, &usuario.Email, &usuario.NombreUso, &usuario.ClaveHash, &nombreRol,
		&usuario.Activo, &usuario.EmailConfirmado, &usuario.CreadoEn, &ultimoIngreso); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncontrado
		}
		return nil, err
	}
	usuario.Rol = Rol(nombreRol)
	if ultimoIngreso.Valid {
		momento := ultimoIngreso.Time
		usuario.UltimoIngreso = &momento
	}
	return &usuario, nil
}
