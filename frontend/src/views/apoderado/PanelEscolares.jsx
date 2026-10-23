import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  cambiarAutorizacion,
  historialAutorizaciones,
  listarVinculaciones,
} from '../../controllers/apoderado/apiApoderado.js';

function formatearFecha(valor) {
  if (!valor) return '—';
  const fecha = new Date(valor);
  if (Number.isNaN(fecha.getTime())) return '—';
  return fecha.toLocaleString('es-PE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export default function PanelEscolares() {
  const [vinculaciones, setVinculaciones] = useState([]);
  const [seleccionadoID, setSeleccionadoID] = useState('');
  const [historiales, setHistoriales] = useState({});
  const [cargandoHistorial, setCargandoHistorial] = useState('');
  const [mensaje, setMensaje] = useState('');
  const [cargando, setCargando] = useState(true);
  const [cambiando, setCambiando] = useState('');

  async function cargarHistorial(escolarID) {
    setCargandoHistorial(escolarID);
    try {
      const respuesta = await historialAutorizaciones(escolarID);
      setHistoriales((actuales) => ({
        ...actuales,
        [escolarID]: respuesta.autorizaciones || [],
      }));
    } catch {
      setHistoriales((actuales) => ({ ...actuales, [escolarID]: null }));
    } finally {
      setCargandoHistorial('');
    }
  }

  async function cargarVinculaciones() {
    try {
      const respuesta = await listarVinculaciones();
      const lista = respuesta.vinculaciones || [];
      setVinculaciones(lista);
      setSeleccionadoID((actual) =>
        lista.some((vinculacion) => vinculacion.escolar.id === actual) ? actual : ''
      );
    } catch (error) {
      setMensaje(error.message);
    } finally {
      setCargando(false);
    }
  }

  useEffect(() => {
    cargarVinculaciones();
  }, []);

  function seleccionarEscolar(evento) {
    const escolarID = evento.target.value;
    setSeleccionadoID(escolarID);
    setMensaje('');

    const vinculacion = vinculaciones.find(
      (candidata) => candidata.escolar.id === escolarID
    );
    if (
      vinculacion?.estado === 'aceptada' &&
      historiales[escolarID] === undefined
    ) {
      cargarHistorial(escolarID);
    }
  }

  async function alternarAutorizacion(vinculacion) {
    const autorizado = vinculacion.autorizacion?.autorizado === true;
    setCambiando(vinculacion.escolar.id);
    setMensaje('');
    try {
      const respuesta = await cambiarAutorizacion(vinculacion.escolar.id, !autorizado);
      setMensaje(respuesta.mensaje);
      await cargarVinculaciones();
      await cargarHistorial(vinculacion.escolar.id);
    } catch (error) {
      setMensaje(error.message);
    } finally {
      setCambiando('');
    }
  }

  const vinculacion = vinculaciones.find(
    (candidata) => candidata.escolar.id === seleccionadoID
  );
  const autorizado = vinculacion?.autorizacion?.autorizado === true;
  const historial = vinculacion ? historiales[vinculacion.escolar.id] : undefined;

  return (
    <main className="pagina-apoderado">
      <h1>Panel de escolares</h1>
      <p>
        Selecciona un escolar para administrar su autorización de pedir nuevas
        tutorías y consultar el historial de cambios.
      </p>

      {mensaje && <p role="alert">{mensaje}</p>}
      {cargando && <p role="status">Cargando…</p>}

      {!cargando && vinculaciones.length === 0 && (
        <p>
          Todavía no has vinculado a ningún escolar.{' '}
          <Link to="/apoderado/vincular">Vincula al primero</Link>.
        </p>
      )}

      {!cargando && vinculaciones.length > 0 && (
        <div className="selector-escolar">
          <label htmlFor="selector-escolar">Escolar</label>
          <select
            id="selector-escolar"
            value={seleccionadoID}
            onChange={seleccionarEscolar}
          >
            <option value="">Selecciona un escolar…</option>
            {vinculaciones.map((candidata) => (
              <option key={candidata.id} value={candidata.escolar.id}>
                {candidata.escolar.nombre_uso} ({candidata.escolar.email})
                {candidata.estado === 'pendiente' ? ' — pendiente' : ''}
              </option>
            ))}
          </select>
        </div>
      )}

      {!cargando && vinculaciones.length > 0 && !vinculacion && (
        <p>Selecciona un escolar de la lista para ver su información.</p>
      )}

      {!cargando && vinculacion && (
        <section className="escolar-panel" aria-labelledby="detalle-escolar">
          <h2 id="detalle-escolar">{vinculacion.escolar.nombre_uso}</h2>
          <p>{vinculacion.escolar.email}</p>
          <p>
            <small>
              {vinculacion.estado === 'aceptada'
                ? 'Vinculación aceptada'
                : 'Pendiente de aceptación'}
            </small>
          </p>

          {vinculacion.estado === 'aceptada' && (
            <>
              <p>
                <small>
                  {autorizado
                    ? 'Puede solicitar nuevas tutorías.'
                    : 'No puede solicitar nuevas tutorías.'}
                </small>
              </p>
              <button
                type="button"
                onClick={() => alternarAutorizacion(vinculacion)}
                disabled={cambiando === vinculacion.escolar.id}
              >
                {cambiando === vinculacion.escolar.id
                  ? 'Guardando…'
                  : autorizado
                    ? 'Revocar autorización'
                    : 'Autorizar nuevas tutorías'}
              </button>

              <section
                className="historial-autorizaciones"
                aria-labelledby="historial-autorizaciones"
              >
                <h3 id="historial-autorizaciones">
                  Historial de autorizaciones y revocaciones
                </h3>
                {historial === undefined ||
                cargandoHistorial === vinculacion.escolar.id ? (
                  <p role="status">Cargando historial…</p>
                ) : historial === null ? (
                  <p>No se pudo cargar el historial.</p>
                ) : historial.length === 0 ? (
                  <p>Sin cambios registrados.</p>
                ) : (
                  <table>
                    <caption>
                      Cambios de permiso para {vinculacion.escolar.nombre_uso}
                    </caption>
                    <thead>
                      <tr>
                        <th scope="col">Fecha y hora</th>
                        <th scope="col">Cambio</th>
                      </tr>
                    </thead>
                    <tbody>
                      {historial.map((autorizacion) => (
                        <tr key={autorizacion.id}>
                          <td>
                            <time dateTime={autorizacion.creada_en}>
                              {formatearFecha(autorizacion.creada_en)}
                            </time>
                          </td>
                          <td>
                            {autorizacion.autorizado
                              ? 'Se otorgó el permiso para pedir tutorías'
                              : 'Se retiró el permiso para pedir tutorías'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </section>
            </>
          )}
        </section>
      )}

      <p>
        <Link to="/apoderado/vincular">Vincular otro escolar</Link>
      </p>
      <p>
        <Link to="/">Volver al inicio</Link>
      </p>
    </main>
  );
}
