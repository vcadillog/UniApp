import { useState } from 'react';
import { Link } from 'react-router-dom';
import Campo from '../../components/Campo.jsx';
import { solicitarVinculacion } from '../../controllers/apoderado/apiApoderado.js';

const FORMULARIO_INICIAL = { email_escolar: '' };

export default function VincularEscolar() {
  const [formulario, setFormulario] = useState(FORMULARIO_INICIAL);
  const [errores, setErrores] = useState({});
  const [mensaje, setMensaje] = useState('');
  const [enviando, setEnviando] = useState(false);

  const cambiarCampo = (evento) =>
    setFormulario({ ...formulario, email_escolar: evento.target.value });

  async function enviarFormulario(evento) {
    evento.preventDefault();
    const email = formulario.email_escolar.trim();
    if (!email) {
      setErrores({ email_escolar: 'Ingresa el correo del escolar.' });
      return;
    }
    setErrores({});
    setMensaje('');
    setEnviando(true);
    try {
      await solicitarVinculacion(email);
      setFormulario(FORMULARIO_INICIAL);
      setMensaje('Solicitud enviada. El escolar recibirá un enlace válido por 24 horas.');
    } catch (error) {
      setMensaje(error.message);
      setErrores(error.campos || {});
    } finally {
      setEnviando(false);
    }
  }

  return (
    <main>
      <h1>Vincular a un escolar</h1>
      <p>
        Ingresa el correo del escolar. Le enviaremos un enlace para que acepte la
        vinculación. El enlace vence en 24 horas.
      </p>
      <form onSubmit={enviarFormulario} noValidate>
        <Campo
          etiqueta="Correo del escolar"
          type="email"
          autoComplete="email"
          value={formulario.email_escolar}
          onChange={cambiarCampo}
          error={errores.email_escolar}
        />
        {mensaje && <p role="alert">{mensaje}</p>}
        <button disabled={enviando}>
          {enviando ? 'Enviando…' : 'Enviar solicitud'}
        </button>
      </form>

      <p>
        <Link to="/panel_escolares">Ir al panel de escolares</Link>
      </p>
      <p>
        <Link to="/">Volver al inicio</Link>
      </p>
    </main>
  );
}
