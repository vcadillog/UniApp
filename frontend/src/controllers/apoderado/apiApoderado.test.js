import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  aceptarVinculacion,
  cambiarAutorizacion,
  consultarVinculacion,
  historialAutorizaciones,
  listarVinculaciones,
  miApoderado,
  solicitarVinculacion,
} from './apiApoderado.js';

afterEach(() => {
  vi.unstubAllGlobals();
});

function simularRespuesta(estado, datos) {
  return {
    ok: estado >= 200 && estado < 300,
    status: estado,
    text: async () => JSON.stringify(datos),
  };
}

function espiarFetch(estado, datos) {
  const fetchSimulado = vi.fn().mockResolvedValue(simularRespuesta(estado, datos));
  vi.stubGlobal('fetch', fetchSimulado);
  return fetchSimulado;
}

describe('I06: cliente API de vinculación', () => {
  it('solicita la vinculación con el correo del escolar', async () => {
    const fetchSimulado = espiarFetch(202, { mensaje: 'enviado' });

    await solicitarVinculacion('escolar@example.com');

    expect(fetchSimulado).toHaveBeenCalledWith(
      '/api/apoderado/vinculaciones',
      expect.objectContaining({
        method: 'POST',
        credentials: 'same-origin',
        body: JSON.stringify({ email_escolar: 'escolar@example.com' }),
      })
    );
  });

  it('lista las vinculaciones del apoderado', async () => {
    const fetchSimulado = espiarFetch(200, { vinculaciones: [] });

    await listarVinculaciones();

    expect(fetchSimulado).toHaveBeenCalledWith(
      '/api/apoderado/vinculaciones',
      expect.objectContaining({ method: 'GET', credentials: 'same-origin' })
    );
  });

  it('consulta y acepta una solicitud por token', async () => {
    const fetchSimulado = espiarFetch(200, { vinculacion: { id: 'v-1' } });

    await consultarVinculacion('token-secreto');
    await aceptarVinculacion('token-secreto');

    expect(fetchSimulado).toHaveBeenNthCalledWith(
      1,
      '/api/escolar/vinculaciones/consultar',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ token: 'token-secreto' }),
      })
    );
    expect(fetchSimulado).toHaveBeenNthCalledWith(
      2,
      '/api/escolar/vinculaciones/aceptar',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ token: 'token-secreto' }),
      })
    );
  });

  it('consulta el apoderado actual del escolar', async () => {
    const fetchSimulado = espiarFetch(200, { vinculacion: null });

    await miApoderado();

    expect(fetchSimulado).toHaveBeenCalledWith(
      '/api/escolar/apoderado',
      expect.objectContaining({ method: 'GET' })
    );
  });

  it('cambia la autorización de un escolar', async () => {
    const fetchSimulado = espiarFetch(200, { autorizacion: { estado: 'no_autorizado' } });

    await cambiarAutorizacion('escolar-1', false);

    expect(fetchSimulado).toHaveBeenCalledWith(
      '/api/apoderado/escolares/escolar-1/autorizacion',
      expect.objectContaining({
        method: 'POST',
        credentials: 'same-origin',
        body: JSON.stringify({ autorizado: false }),
      })
    );
  });

  it('consulta el historial de autorizaciones de un escolar', async () => {
    const fetchSimulado = espiarFetch(200, { autorizaciones: [] });

    await historialAutorizaciones('escolar-1');

    expect(fetchSimulado).toHaveBeenCalledWith(
      '/api/apoderado/escolares/escolar-1/autorizaciones',
      expect.objectContaining({ method: 'GET', credentials: 'same-origin' })
    );
  });

  it('propaga los campos de validación del backend', async () => {
    espiarFetch(422, {
      codigo: 'validacion',
      mensaje: 'Revisa los datos ingresados.',
      campos: { email_escolar: 'el correo es obligatorio' },
    });

    await expect(solicitarVinculacion('')).rejects.toMatchObject({
      estado: 422,
      campos: { email_escolar: 'el correo es obligatorio' },
    });
  });
});
