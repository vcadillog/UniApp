import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import Campo from '../../components/Campo.jsx';
import { ingresar } from '../../controllers/acceso/acciones.js';
import { validarIngreso } from '../../models/acceso/validadores.js';

const FORMULARIO_INICIAL = { email: '', clave: '' };

export default function Ingreso() {
  const [formulario, setFormulario] = useState(FORMULARIO_INICIAL);
  const [errores, setErrores] = useState({});
  const [mensaje, setMensaje] = useState('');
  const [enviando, setEnviando] = useState(false);
  const navegar = useNavigate();
  const [parametros] = useSearchParams();
  const solicitado = parametros.get('next');
  const esDestinoInterno = solicitado?.startsWith('/') && !solicitado.startsWith('//') && !solicitado.startsWith('/\\');
  const destino = esDestinoInterno ? solicitado : '/';
  const cambiarCampo = (campo) => (evento) =>
    setFormulario({ ...formulario, [campo]: evento.target.value });

  async function enviarFormulario(evento) {
    evento.preventDefault();
    const erroresValidacion = validarIngreso(formulario);
    setErrores(erroresValidacion);
    setMensaje('');
    if (Object.keys(erroresValidacion).length) return;
    setEnviando(true);
    try {
      await ingresar(formulario.email.trim(), formulario.clave);
      navegar(destino, { replace: true });
    } catch (error) {
      setMensaje(error.message);
    } finally {
      setEnviando(false);
    }
  }

  return (
    <main>
      <h1>Ingresar</h1>
      {solicitado && <p>Inicia sesión para continuar.</p>}
      <form onSubmit={enviarFormulario} noValidate>
        <Campo etiqueta="Correo" type="email" autoComplete="username" value={formulario.email} onChange={cambiarCampo('email')} error={errores.email} />
        <Campo etiqueta="Contraseña" type="password" autoComplete="current-password" value={formulario.clave} onChange={cambiarCampo('clave')} error={errores.clave} />
        {mensaje && <p className="error" role="alert">{mensaje}</p>}
        <button disabled={enviando}>{enviando ? 'Ingresando…' : 'Ingresar'}</button>
      </form>
      <p><Link to="/recuperar">¿Olvidaste tu contraseña?</Link> · <Link to="/registro">Crear cuenta</Link></p>
    </main>
  );
}
