import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { miApoderado } from '../../controllers/apoderado/apiApoderado.js';

export default function MiApoderado() {
  const [vinculacion, setVinculacion] = useState(null);
  const [cargando, setCargando] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    miApoderado()
      .then((respuesta) => setVinculacion(respuesta.vinculacion))
      .catch((causa) => setError(causa.message))
      .finally(() => setCargando(false));
  }, []);

  return (
    <main>
      <h1>Mi apoderado</h1>
      {cargando && <p role="status">Cargando…</p>}
      {error && <p className="error" role="alert">{error}</p>}
      {!cargando && !error && !vinculacion && (
        <p>Todavía no tienes un apoderado vinculado.</p>
      )}
      {vinculacion && (
        <>
          <p>
            <strong>{vinculacion.apoderado.nombre_uso}</strong>
            <br />
            {vinculacion.apoderado.email}
          </p>
          <p>
            La vinculación fue aceptada y no se puede desvincular.
          </p>
          {vinculacion.autorizacion && (
            <p role="status">
              {vinculacion.autorizacion.autorizado
                ? 'Tu apoderado autorizó la creación de nuevas tutorías.'
                : 'Tu apoderado revocó la autorización para crear nuevas tutorías.'}
              <br />
              <small>
                Estado registrado el{' '}
                {new Date(vinculacion.autorizacion.creada_en).toLocaleString('es-PE')}.
                Las tutorías ya confirmadas no se modifican.
              </small>
            </p>
          )}
        </>
      )}
      <p><Link to="/">Volver al inicio</Link></p>
    </main>
  );
}
