import { api } from '../acceso/acciones.js';

export const solicitarVinculacion = (emailEscolar) =>
  api.post('/api/apoderado/vinculaciones', { email_escolar: emailEscolar });

export const listarVinculaciones = () => api.get('/api/apoderado/vinculaciones');

export const cambiarAutorizacion = (escolarID, autorizado) =>
  api.post(`/api/apoderado/escolares/${encodeURIComponent(escolarID)}/autorizacion`, {
    autorizado,
  });

export const historialAutorizaciones = (escolarID) =>
  api.get(`/api/apoderado/escolares/${encodeURIComponent(escolarID)}/autorizaciones`);

export const consultarVinculacion = (token) =>
  api.post('/api/escolar/vinculaciones/consultar', { token });

export const aceptarVinculacion = (token) =>
  api.post('/api/escolar/vinculaciones/aceptar', { token });

export const miApoderado = () => api.get('/api/escolar/apoderado');
