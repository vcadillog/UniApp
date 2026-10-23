import { politicaDeRol, requiereCorreoInstitucional } from './roles.js';

const MINIMO_BASICA = 10;
const MINIMO_ESTRICTA = 12;
const CORREO_VALIDO = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const DOMINIOS_INSTITUCIONALES = 'uni.edu.pe,uni.pe';

export const politicaBasica = (minimo = MINIMO_BASICA) => (clave, email = '') => {
  const observaciones = [];
  if ([...clave].length < minimo) observaciones.push(`debe tener al menos ${minimo} caracteres`);
  const nombreLocal = email.split('@')[0].toLowerCase();
  if (nombreLocal && clave.toLowerCase() === nombreLocal) observaciones.push('no puede ser igual a tu correo');
  return observaciones;
};

export const politicaEstricta = (minimo = MINIMO_ESTRICTA) => (clave, email = '') => {
  const observaciones = politicaBasica(minimo)(clave, email);
  const combinaClases = /[A-Z]/.test(clave) && /[a-z]/.test(clave) && /\d/.test(clave) && /[^A-Za-z0-9]/.test(clave);
  if (!combinaClases) observaciones.push('debe combinar mayúscula, minúscula, número y símbolo');
  return observaciones;
};

const POLITICAS = {
  basica: () => politicaBasica(MINIMO_BASICA),
  estricta: () => politicaEstricta(MINIMO_ESTRICTA),
};

export const crearPolitica = (rol) => (POLITICAS[politicaDeRol(rol)] ?? POLITICAS.basica)();

export function dominiosDe(dominios) {
  return String(dominios ?? '')
    .split(',')
    .map((dominio) => dominio.trim().toLowerCase())
    .filter(Boolean);
}

export function correosInstitucionales(dominios = DOMINIOS_INSTITUCIONALES) {
  const correos = dominiosDe(dominios).map((dominio) => '@' + dominio);
  return correos.length ? correos.join(' o ') : 'correo institucional';
}

function esCorreoInstitucional(email, dominios) {
  return dominiosDe(dominios).some((dominio) => email.endsWith('@' + dominio));
}

function validarCorreo(email) {
  return CORREO_VALIDO.test(email) ? '' : 'Correo no válido';
}

export function validarRegistro(datos, dominiosTutor = DOMINIOS_INSTITUCIONALES) {
  const errores = {};
  const email = (datos.email || '').trim().toLowerCase();
  const problemaCorreo = validarCorreo(email);
  if (problemaCorreo) {
    errores.email = problemaCorreo;
  } else if (requiereCorreoInstitucional(datos.rol) && !esCorreoInstitucional(email, dominiosTutor)) {
    errores.email = `Los tutores usan su correo institucional (${correosInstitucionales(dominiosTutor)})`;
  }
  const longitud = [...(datos.nombre_uso || '').trim()].length;
  if (longitud < 2 || longitud > 60) errores.nombre_uso = 'Entre 2 y 60 caracteres';
  const observaciones = crearPolitica(datos.rol)(datos.clave || '', email);
  if (observaciones.length) errores.clave = 'La contraseña ' + observaciones.join('; ');
  if (datos.clave !== datos.confirmar) errores.confirmar = 'Las contraseñas no coinciden';
  return errores;
}

export function validarIngreso(datos) {
  const errores = {};
  const problemaCorreo = validarCorreo((datos.email || '').trim());
  if (problemaCorreo) errores.email = problemaCorreo;
  if (!datos.clave) errores.clave = 'Ingresa tu contraseña';
  return errores;
}

export function validarRecuperar(datos) {
  const errores = {};
  const problemaCorreo = validarCorreo((datos.email || '').trim());
  if (problemaCorreo) errores.email = problemaCorreo;
  return errores;
}

export function validarRestablecer(datos) {
  const errores = {};
  const observaciones = politicaBasica(MINIMO_BASICA)(datos.clave || '');
  if (observaciones.length) errores.clave = 'La contraseña ' + observaciones.join('; ');
  if (datos.clave !== datos.confirmar) errores.confirmar = 'Las contraseñas no coinciden';
  return errores;
}
