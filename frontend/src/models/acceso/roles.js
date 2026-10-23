export const ROLES = {
  escolar: { etiqueta: 'Escolar', registro: true, politica: 'basica', correoInstitucional: false },
  apoderado: { etiqueta: 'Apoderado', registro: true, politica: 'basica', correoInstitucional: false },
  tutor: { etiqueta: 'Tutor UNI', registro: true, politica: 'estricta', correoInstitucional: true },
  coordinador: { etiqueta: 'Coordinación', registro: false, politica: 'estricta', correoInstitucional: false },
};

export const ROLES_REGISTRO = Object.entries(ROLES)
  .filter(([, definicion]) => definicion.registro)
  .map(([rol]) => rol);

export const etiquetaRol = (rol) => ROLES[rol]?.etiqueta ?? rol;

export const politicaDeRol = (rol) => ROLES[rol]?.politica ?? 'basica';

export const requiereCorreoInstitucional = (rol) => Boolean(ROLES[rol]?.correoInstitucional);

export const puedeAcceder = (rol, permitidos) => !permitidos || permitidos.includes(rol);
