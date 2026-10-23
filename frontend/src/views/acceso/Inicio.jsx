import { Link } from 'react-router-dom';
import { salir } from '../../controllers/acceso/acciones.js';
import { useSesion } from '../../controllers/acceso/useSesion.js';
import { etiquetaRol } from '../../models/acceso/roles.js';

export default function Inicio() {
  const { usuario } = useSesion();
  return (
    <main>
      <h1>Hola, {usuario.nombre_uso}</h1>
      <p>Rol: <strong>{etiquetaRol(usuario.rol)}</strong></p>
      <ul>
        {import.meta.env.VITE_I05_DEMO === 'true' && ['tutor', 'escolar'].includes(usuario.rol) && (
          <li><Link to="/sesiones/sesion-i05-demo">Ver sesión de demostración</Link></li>
        )}
        {usuario.rol === 'coordinador' && <li><Link to="/coordinacion">Zona de coordinación</Link></li>}
        {usuario.rol === 'escolar' && <li><Link to="/reservar">Reservar una tutoría</Link></li>}
        {usuario.rol === 'escolar' && <li><Link to="/mi-apoderado">Mi apoderado</Link></li>}
        {usuario.rol === 'tutor' && <li><Link to="/expediente">Mi expediente de tutor</Link></li>}
        {usuario.rol === 'tutor' && <li><Link to="/tutor/disponibilidad">Gestionar disponibilidad semanal</Link></li>}
        {usuario.rol === 'apoderado' && <li><Link to="/panel_escolares">Panel de escolares</Link></li>}
        {usuario.rol === 'apoderado' && <li><Link to="/apoderado/vincular">Vincular a un escolar</Link></li>}
      </ul>
      <button onClick={salir}>Cerrar sesión</button>
    </main>
  );
}
