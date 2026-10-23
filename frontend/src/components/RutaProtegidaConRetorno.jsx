import { Navigate, useLocation } from 'react-router-dom';
import RutaProtegida from './RutaProtegida.jsx';
import { useSesion } from '../controllers/acceso/useSesion.js';

/**
 * Extiende RutaProtegida sin modificarla: si no hay sesión, conserva el
 * destino completo (incluido el token de un enlace) para volver tras el login.
 */
export default function RutaProtegidaConRetorno({ roles, children }) {
  const { usuario, cargando } = useSesion();
  const ubicacion = useLocation();

  if (!cargando && !usuario) {
    const siguiente = encodeURIComponent(`${ubicacion.pathname}${ubicacion.search}`);
    return <Navigate to={`/ingreso?next=${siguiente}`} replace />;
  }

  return (
    <RutaProtegida roles={roles}>
      {children}
    </RutaProtegida>
  );
}
