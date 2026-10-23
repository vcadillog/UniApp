import { ErrorApi } from './apiCliente.js';

export function conSesionExpirada(cliente, alExpirar) {
  const vigilar = (operacion) => async (ruta, ...argumentos) => {
    try {
      return await operacion(ruta, ...argumentos);
    } catch (error) {
      const esSesionExpirada = error instanceof ErrorApi && error.estado === 401;
      const esIngreso = ruta.startsWith('/api/auth/ingreso');
      if (esSesionExpirada && !esIngreso) alExpirar();
      throw error;
    }
  };
  return {
    get: vigilar(cliente.get),
    post: vigilar(cliente.post),
    postFormulario: vigilar(cliente.postFormulario),
    put: vigilar(cliente.put),
    delete: vigilar(cliente.delete),
  };
}
