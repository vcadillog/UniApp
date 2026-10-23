// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '../../App.jsx';
import Inicio from '../acceso/Inicio.jsx';
import { obtenerSesion, declararAsistencia } from '../../controllers/sesion/apiSesion.js';
const estado = vi.hoisted(() => ({ usuario: null, cargando: false }));
vi.mock('../../controllers/acceso/useSesion.js', () => ({ useSesion: () => estado }));
vi.mock('../../controllers/acceso/acciones.js', () => ({ cargarSesion: vi.fn(), salir: vi.fn() }));
vi.mock('../../controllers/sesion/apiSesion.js', () => ({ obtenerSesion: vi.fn(), declararAsistencia: vi.fn() }));
const sesion = { id: 'sesion-i05-demo', reserva_id: 'reserva-i05-demo', tutor_id: 'tutor-001', escolar_id: 'escolar-001', inicio: '2026-10-20T15:00:00Z', fin: '2026-10-20T16:00:00Z', asistencias: [] };
const asistencia = rol => ({ usuario_id: rol + '-001', rol, declarada_en: sesion.inicio });
function abrir(ruta = '/sesiones/sesion-i05-demo') { window.history.replaceState({}, '', ruta); return render(<App />); }
beforeEach(() => {
  vi.resetAllMocks();
  estado.usuario = { id: 'tutor-001', rol: 'tutor', nombre_uso: 'Tutor Prueba' };
  obtenerSesion.mockResolvedValue(sesion);
});
afterEach(() => { cleanup(); vi.unstubAllEnvs(); });
describe('Detalle I05 y acceso por ruta real', () => {
  it('muestra carga, participantes pendientes, horario de Lima y demostración', async () => {
    abrir();
    expect(screen.getByRole('status').textContent).toContain('Cargando');
    await screen.findByRole('heading', { name: 'Detalle de tutoría' });
    expect(screen.getByText('Tutor:')).toBeTruthy();
    expect(screen.getByText('Escolar:')).toBeTruthy();
    expect(screen.getAllByText('Pendiente')).toHaveLength(2);
    expect(screen.getByText(/Sesión sintética/)).toBeTruthy();
    const fecha = new Intl.DateTimeFormat('es-PE', { timeZone: 'America/Lima', dateStyle: 'medium', timeStyle: 'short' }).format(new Date(sesion.inicio));
    expect(screen.getByText(fecha, { exact: false })).toBeTruthy();
    expect(screen.getByText('60 minutos', { exact: false, selector: 'p' })).toBeTruthy();
    expect(obtenerSesion).toHaveBeenCalledWith(sesion.id);
  });
  it.each(['tutor', 'escolar'])('%s declara solo su asistencia y vuelve a cargarla', async rol => {
    estado.usuario = { id: rol + '-001', rol, nombre_uso: rol };
    const guardada = { ...sesion, asistencias: [asistencia(rol)] };
    obtenerSesion.mockResolvedValueOnce(sesion).mockResolvedValue(guardada);
    declararAsistencia.mockResolvedValue(asistencia(rol));
    const vista = abrir();
    fireEvent.click(await screen.findByRole('button', { name: 'Declarar mi asistencia' }));
    const boton = await screen.findByRole('button', { name: 'Asistencia ya declarada' });
    expect(boton.disabled).toBe(true);
    fireEvent.click(boton);
    expect(declararAsistencia).toHaveBeenCalledTimes(1);
    expect(declararAsistencia).toHaveBeenCalledWith(sesion.id);
    expect(await screen.findByText('Tu asistencia fue declarada correctamente.')).toBeTruthy();
    vista.unmount();
    abrir();
    expect((await screen.findByRole('button', { name: 'Asistencia ya declarada' })).disabled).toBe(true);
  });
  it('no duplica peticiones mientras se registra', async () => {
    declararAsistencia.mockImplementation(() => new Promise(() => {}));
    abrir();
    const boton = await screen.findByRole('button', { name: 'Declarar mi asistencia' });
    fireEvent.click(boton); fireEvent.click(boton);
    expect(boton.disabled).toBe(true);
    expect(declararAsistencia).toHaveBeenCalledTimes(1);
  });
  it('mantiene deshabilitada la asistencia ya guardada aunque falle la recarga', async () => {
    obtenerSesion.mockResolvedValueOnce(sesion).mockRejectedValue(new Error('Sin red'));
    declararAsistencia.mockResolvedValue(asistencia('tutor'));
    abrir();
    fireEvent.click(await screen.findByRole('button', { name: 'Declarar mi asistencia' }));
    expect((await screen.findByRole('alert')).textContent).toContain('Tu asistencia se guardó');
    expect(screen.getByRole('button', { name: 'Asistencia ya declarada' }).disabled).toBe(true);
  });
  it('muestra ambas declaraciones persistidas', async () => {
    obtenerSesion.mockResolvedValue({ ...sesion, asistencias: [asistencia('tutor'), asistencia('escolar')] });
    abrir();
    expect(await screen.findAllByText('Declarada')).toHaveLength(2);
    expect(screen.getByRole('button', { name: 'Asistencia ya declarada' }).disabled).toBe(true);
  });
  it('muestra el rechazo de una declaración duplicada del servidor', async () => {
    declararAsistencia.mockRejectedValue(new Error('Asistencia ya declarada en otra pestaña.'));
    abrir();
    fireEvent.click(await screen.findByRole('button', { name: 'Declarar mi asistencia' }));
    expect((await screen.findByRole('alert')).textContent).toContain('otra pestaña');
    expect(screen.queryByText('Tu asistencia fue declarada correctamente.')).toBeNull();
  });
  it('muestra error de consulta privada y enlace de retorno', async () => {
    obtenerSesion.mockRejectedValue(new Error('No participas en esta sesión.'));
    abrir();
    expect((await screen.findByRole('alert')).textContent).toContain('No participas');
    expect(screen.getByRole('link', { name: 'Volver al inicio' }).getAttribute('href')).toBe('/');
  });
  it('no etiqueta otras sesiones como demostración', async () => {
    obtenerSesion.mockResolvedValue({ ...sesion, id: 'otra' });
    abrir('/sesiones/otra');
    await screen.findByRole('heading', { name: 'Detalle de tutoría' });
    expect(screen.queryByText(/Sesión sintética/)).toBeNull();
  });
  it.each(['apoderado', 'coordinador'])('bloquea la ruta al %s', async rol => {
    estado.usuario = { id: 'otro', rol };
    abrir();
    await waitFor(() => expect(window.location.pathname).toBe('/no-autorizado'));
    expect(obtenerSesion).not.toHaveBeenCalled();
  });
  it('redirige al ingreso sin autenticación', async () => {
    estado.usuario = null; abrir();
    await waitFor(() => expect(window.location.pathname).toBe('/ingreso'));
    expect(obtenerSesion).not.toHaveBeenCalled();
  });
});
describe('Inicio y modo demo', () => {
  it.each(['tutor', 'escolar'])('permite navegar al %s en modo demo', async rol => {
    vi.stubEnv('VITE_I05_DEMO', 'true');
    estado.usuario = { id: rol + '-001', rol, nombre_uso: rol };
    abrir('/');
    fireEvent.click(screen.getByRole('link', { name: 'Ver sesión de demostración' }));
    await screen.findByRole('heading', { name: 'Detalle de tutoría' });
    expect(window.location.pathname).toBe('/sesiones/sesion-i05-demo');
  });
  it.each(['false', '', undefined])('oculta demo cuando la opción vale %s', valor => {
    vi.stubEnv('VITE_I05_DEMO', valor);
    render(<MemoryRouter><Inicio /></MemoryRouter>);
    expect(screen.queryByRole('link', { name: 'Ver sesión de demostración' })).toBeNull();
  });
  it.each(['apoderado', 'coordinador'])('no ofrece demo al %s', rol => {
    vi.stubEnv('VITE_I05_DEMO', 'true'); estado.usuario = { id: 'otro', rol };
    render(<MemoryRouter><Inicio /></MemoryRouter>);
    expect(screen.queryByRole('link', { name: 'Ver sesión de demostración' })).toBeNull();
  });
  it.each([['tutor', ['Mi expediente de tutor', 'Gestionar disponibilidad semanal']], ['escolar', ['Reservar una tutoría']], ['coordinador', ['Zona de coordinación']]])('conserva navegación del %s', (rol, enlaces) => {
    estado.usuario = { id: 'otro', rol }; render(<MemoryRouter><Inicio /></MemoryRouter>);
    enlaces.forEach(name => expect(screen.getByRole('link', { name })).toBeTruthy());
  });
});
