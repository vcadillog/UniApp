import { describe, expect, it, vi } from 'vitest';
import { ROLES, ROLES_REGISTRO, etiquetaRol, politicaDeRol, puedeAcceder, requiereCorreoInstitucional } from './roles.js';
import { crearPolitica, validarRegistro } from './validadores.js';
import { crearAlmacen } from '../../controllers/acceso/almacenSesion.js';
import { crearClienteHttp, ErrorApi } from '../../controllers/acceso/apiCliente.js';
import { conSesionExpirada } from '../../controllers/acceso/decoradores.js';

describe('modelo: catálogo de roles (registro centralizado)', () => {
  it('deriva los roles públicos y expone sus metadatos', () => {
    expect(ROLES_REGISTRO).toEqual(['escolar', 'apoderado', 'tutor']);
    expect(ROLES_REGISTRO).not.toContain('coordinador');
    for (const rol of Object.keys(ROLES)) {
      expect(etiquetaRol(rol)).toBeTruthy();
      expect(['basica', 'estricta']).toContain(politicaDeRol(rol));
    }
    expect(etiquetaRol('desconocido')).toBe('desconocido');
    expect(politicaDeRol('desconocido')).toBe('basica');
    expect(requiereCorreoInstitucional('tutor')).toBe(true);
    expect(requiereCorreoInstitucional('escolar')).toBe(false);
    expect(requiereCorreoInstitucional('desconocido')).toBe(false);
  });
});

describe('modelo: validadores (Strategy + Factory)', () => {
  it('tutor usa política estricta y escolar la básica', () => {
    expect(crearPolitica('escolar')('larga-pero-simple')).toEqual([]);
    expect(crearPolitica('tutor')('larga-pero-simple').length).toBeGreaterThan(0);
  });
  it('tutor exige correo institucional', () => {
    const base = { nombre_uso: 'Tito', clave: 'Clave-Segura-2026!', confirmar: 'Clave-Segura-2026!', rol: 'tutor' };
    expect(validarRegistro({ ...base, email: 't@gmail.com' }).email).toBeTruthy();
    expect(validarRegistro({ ...base, email: 't@uni.edu.pe.evil.com' }).email).toBeTruthy();
    expect(validarRegistro({ ...base, email: 't@uni.edu.pe' })).toEqual({});
    expect(validarRegistro({ ...base, email: 't@uni.pe' })).toEqual({});
  });
  it('detecta contraseñas distintas', () => {
    const e = validarRegistro({ email: 'a@b.co', nombre_uso: 'Ana', clave: 'clave-larga-ok', confirmar: 'otra', rol: 'escolar' });
    expect(e.confirmar).toBeTruthy();
  });
  it('un escolar no puede acceder a rutas de coordinación', () => {
    expect(puedeAcceder('escolar', ['coordinador'])).toBe(false);
    expect(puedeAcceder('coordinador', ['coordinador'])).toBe(true);
    expect(puedeAcceder('escolar', undefined)).toBe(true);
  });
});

describe('controlador: almacén (Observer)', () => {
  it('notifica a suscriptores y permite darse de baja', () => {
    const a = crearAlmacen();
    const fn = vi.fn();
    const baja = a.subscribe(fn);
    a.set({ usuario: { rol: 'escolar' }, cargando: false });
    expect(fn).toHaveBeenCalledTimes(1);
    baja();
    a.set({ usuario: null });
    expect(fn).toHaveBeenCalledTimes(1);
  });
});

describe('controlador: cliente (Adapter + Decorator)', () => {
  const respuesta = (estado, cuerpo) => ({ ok: estado < 400, status: estado, text: async () => (cuerpo ? JSON.stringify(cuerpo) : '') });

  it('normaliza errores HTTP y de red en ErrorApi', async () => {
    const c = crearClienteHttp(async () => respuesta(422, { codigo: 'validacion', mensaje: 'Revisa', campos: { clave: 'x' } }));
    await expect(c.post('/x', {})).rejects.toMatchObject({ estado: 422, campos: { clave: 'x' } });
    const caido = crearClienteHttp(async () => { throw new Error('boom'); });
    await expect(caido.get('/x')).rejects.toMatchObject({ estado: 0, codigo: 'red' });
  });

  it('el decorador conserva postFormulario del cliente decorado', async () => {
    const formulario = new FormData();
    const base = {
      get: vi.fn(),
      post: vi.fn(),
      postFormulario: vi.fn(async () => ({ adjunto_id: 'a-1' })),
      put: vi.fn(),
      delete: vi.fn(),
    };
    const c = conSesionExpirada(base, vi.fn());
    await expect(c.postFormulario('/api/reservas/r-1/adjuntos', formulario)).resolves.toEqual({ adjunto_id: 'a-1' });
    expect(base.postFormulario).toHaveBeenCalledWith('/api/reservas/r-1/adjuntos', formulario);
  });

  it('el decorador avisa cuando una ruta protegida devuelve 401, pero no en el ingreso', async () => {
    const base = crearClienteHttp(async () => respuesta(401, { mensaje: 'no' }));
    const alExpirar = vi.fn();
    const c = conSesionExpirada(base, alExpirar);
    await expect(c.get('/api/auth/yo')).rejects.toBeInstanceOf(ErrorApi);
    expect(alExpirar).toHaveBeenCalledTimes(1);
    await expect(c.post('/api/auth/ingreso', {})).rejects.toBeInstanceOf(ErrorApi);
    expect(alExpirar).toHaveBeenCalledTimes(1);
  });
});

describe('controlador: acciones de confirmación (Adapter)', () => {
  it('envía el token y el correo a los endpoints correctos', async () => {
    const llamadas = [];
    vi.stubGlobal('fetch', vi.fn(async (url, opciones) => {
      llamadas.push({ url, opciones });
      return { ok: true, status: 200, text: async () => JSON.stringify({ mensaje: 'ok' }) };
    }));
    const { confirmar, reenviarConfirmacion } = await import('../../controllers/acceso/acciones.js');
    await confirmar('tok-123');
    await reenviarConfirmacion('ana@uni.pe');
    vi.unstubAllGlobals();
    expect(llamadas.map((llamada) => llamada.url)).toEqual([
      '/api/auth/confirmar',
      '/api/auth/reenviar-confirmacion',
    ]);
    expect(JSON.parse(llamadas[0].opciones.body)).toEqual({ token: 'tok-123' });
    expect(JSON.parse(llamadas[1].opciones.body)).toEqual({ email: 'ana@uni.pe' });
    expect(llamadas[0].opciones.credentials).toBe('same-origin');
  });
});

describe('controlador: panel de coordinación (Adapter)', () => {
  it('consulta el endpoint del panel con la sesión', async () => {
    const llamadas = [];
    vi.stubGlobal('fetch', vi.fn(async (url, opciones) => {
      llamadas.push({ url, opciones });
      return { ok: true, status: 200, text: async () => JSON.stringify({ mensaje: 'Panel de coordinación' }) };
    }));
    const { panelCoordinacion } = await import('../../controllers/acceso/acciones.js');
    await panelCoordinacion();
    vi.unstubAllGlobals();
    expect(llamadas).toHaveLength(1);
    expect(llamadas[0].url).toBe('/api/coordinacion/panel');
    expect(llamadas[0].opciones.method).toBe('GET');
    expect(llamadas[0].opciones.credentials).toBe('same-origin');
  });
});
