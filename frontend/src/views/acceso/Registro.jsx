import { useState } from 'react';
import { Link } from 'react-router-dom';
import Campo from '../../components/Campo.jsx';
import { registrar, reenviarConfirmacion } from '../../controllers/acceso/acciones.js';
import { ROLES_REGISTRO, etiquetaRol, requiereCorreoInstitucional } from '../../models/acceso/roles.js';
import { correosInstitucionales, validarRegistro } from '../../models/acceso/validadores.js';

const DOMINIOS_INSTITUCIONALES = import.meta.env?.VITE_TUTOR_EMAIL_DOMAIN || 'uni.edu.pe,uni.pe';

const FORMULARIO_INICIAL = { email: '', nombre_uso: '', clave: '', confirmar: '', rol: 'escolar' };

export default function Registro() {
  const [formulario, setFormulario] = useState(FORMULARIO_INICIAL);
  const [errores, setErrores] = useState({});
  const [mensaje, setMensaje] = useState('');
  const [creado, setCreado] = useState(false);
  const [aviso, setAviso] = useState('');
  const [enviando, setEnviando] = useState(false);
  const cambiarCampo = (campo) => (evento) =>
    setFormulario({ ...formulario, [campo]: evento.target.value });

  async function enviarFormulario(evento) {
    evento.preventDefault();
    const erroresValidacion = validarRegistro(formulario, DOMINIOS_INSTITUCIONALES);
    setErrores(erroresValidacion);
    setMensaje('');
    if (Object.keys(erroresValidacion).length) return;
    try {
      await registrar(formulario);
      setCreado(true);
    } catch (error) {
      setErrores(error.campos || {});
      setMensaje(error.message);
    }
  }

  async function reenviar() {
    setEnviando(true);
    setAviso('');
    try {
      const respuesta = await reenviarConfirmacion(formulario.email.trim());
      setAviso(respuesta.mensaje);
    } catch (error) {
      setAviso(error.message);
    } finally {
      setEnviando(false);
    }
  }

  if (creado) {
    return (
      <main>
        <h1>Revisa tu correo</h1>
        <p>
          Te enviamos un enlace a <strong>{formulario.email}</strong> para confirmar la cuenta.
          Ábrelo para poder ingresar.
        </p>
        <p>
          El enlace vence en <strong>30 minutos</strong>. Si no lo confirmas, el correo
          quedará libre y podrás registrarte de nuevo.
        </p>
        {aviso && <p role="status">{aviso}</p>}
        <p>
          <button type="button" onClick={reenviar} disabled={enviando}>
            {enviando ? 'Enviando…' : 'Reenviar enlace'}
          </button>
        </p>
        <p><Link to="/ingreso">Ya confirmé mi correo</Link></p>
      </main>
    );
  }
  return (
    <main>
      <h1>Crear cuenta</h1>
      <form onSubmit={enviarFormulario} noValidate>
        <div className="campo">
          <label htmlFor="rol">Soy</label>
          <select id="rol" value={formulario.rol} onChange={cambiarCampo('rol')}>
            {ROLES_REGISTRO.map((rol) => <option key={rol} value={rol}>{etiquetaRol(rol)}</option>)}
          </select>
        </div>
        <Campo etiqueta="Nombre de uso" autoComplete="nickname" value={formulario.nombre_uso} onChange={cambiarCampo('nombre_uso')} error={errores.nombre_uso}
          ayuda="No pedimos DNI ni dirección." />
        <Campo etiqueta="Correo" type="email" autoComplete="email" value={formulario.email} onChange={cambiarCampo('email')} error={errores.email}
          ayuda={requiereCorreoInstitucional(formulario.rol) ? `Correo institucional (${correosInstitucionales(DOMINIOS_INSTITUCIONALES)})` : undefined} />
        <Campo etiqueta="Contraseña" type="password" autoComplete="new-password" value={formulario.clave} onChange={cambiarCampo('clave')} error={errores.clave} />
        <Campo etiqueta="Repite la contraseña" type="password" autoComplete="new-password" value={formulario.confirmar} onChange={cambiarCampo('confirmar')} error={errores.confirmar} />
        {mensaje && <p className="error" role="alert">{mensaje}</p>}
        <p><small>Te enviaremos un enlace de confirmación válido por 30 minutos.</small></p>
        <button>Crear cuenta</button>
      </form>
      <p>¿Ya tienes cuenta? <Link to="/ingreso">Ingresar</Link></p>
    </main>
  );
}
