import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  aceptarVinculacion,
  consultarVinculacion,
} from '../../controllers/apoderado/apiApoderado.js';

export default function AceptarVinculacion() {
  const [parametros] = useSearchParams();
  const token = parametros.get('token') || '';
  const [solicitud, setSolicitud] = useState(null);
  const [aceptada, setAceptada] = useState(false);
  const [error, setError] = useState('');
  const [cargando, setCargando] = useState(true);
  const [enviando, setEnviando] = useState(false);

  useEffect(() => {
    if (!token) {
      setError('El enlace no contiene un token de vinculación.');
      setCargando(false);
      return;
    }
    consultarVinculacion(token)
      .then((respuesta) => setSolicitud(respuesta.vinculacion))
      .catch((causa) => setError(causa.message))
      .finally(() => setCargando(false));
  }, [token]);

  async function aceptar() {
    setError('');
    setEnviando(true);
    try {
      const respuesta = await aceptarVinculacion(token);
      setSolicitud(respuesta.vinculacion);
      setAceptada(true);
    } catch (causa) {
      setError(causa.message);
    } finally {
      setEnviando(false);
    }
  }

  if (cargando) return <main><p role="status">Validando enlace…</p></main>;

  return (
    <main>
      <h1>Aceptar vinculación</h1>
      {error && <p className="error" role="alert">{error}</p>}
      {solicitud && (
        <>
          <p>
            <strong>{solicitud.apoderado.nombre_uso}</strong> ({solicitud.apoderado.email})
            quiere vincularse como tu apoderado.
          </p>
          {aceptada ? (
            <p role="status">
              Vinculación aceptada. Tu apoderado ya está registrado y la relación
              no se puede deshacer.
            </p>
          ) : (
            <button onClick={aceptar} disabled={enviando}>
              {enviando ? 'Aceptando…' : 'Aceptar vinculación'}
            </button>
          )}
          <p><Link to="/mi-apoderado">Ver mi apoderado</Link></p>
        </>
      )}
      {!solicitud && !error && <p>No se encontró una solicitud pendiente.</p>}
      <p><Link to="/">Volver al inicio</Link></p>
    </main>
  );
}
