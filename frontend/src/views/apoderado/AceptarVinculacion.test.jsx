// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AceptarVinculacion from './AceptarVinculacion.jsx';
import {
  aceptarVinculacion,
  consultarVinculacion,
} from '../../controllers/apoderado/apiApoderado.js';

vi.mock('../../controllers/apoderado/apiApoderado.js', () => ({
  consultarVinculacion: vi.fn(),
  aceptarVinculacion: vi.fn(),
}));

const pendiente = {
  id: 'v-1',
  estado: 'pendiente',
  apoderado: { nombre_uso: 'Apoderado Prueba', email: 'apoderado@example.com' },
  escolar: { nombre_uso: 'Escolar Prueba', email: 'escolar@example.com' },
};

function renderizar(ruta = '/vinculacion/aceptar?token=token-secreto') {
  return render(
    <MemoryRouter initialEntries={[ruta]}>
      <Routes>
        <Route path="/vinculacion/aceptar" element={<AceptarVinculacion />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('I06: aceptar vinculación', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('muestra al apoderado y acepta el enlace', async () => {
    consultarVinculacion.mockResolvedValue({ vinculacion: pendiente });
    aceptarVinculacion.mockResolvedValue({
      vinculacion: { ...pendiente, estado: 'aceptada' },
    });

    renderizar();

    expect(await screen.findByText('Apoderado Prueba')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Aceptar vinculación' }));

    expect(await screen.findByText(/Vinculación aceptada/)).toBeTruthy();
    expect(consultarVinculacion).toHaveBeenCalledWith('token-secreto');
    expect(aceptarVinculacion).toHaveBeenCalledWith('token-secreto');
  });

  it('avisa cuando el enlace no es válido', async () => {
    consultarVinculacion.mockRejectedValue(new Error('enlace inválido o vencido'));

    renderizar();

    expect((await screen.findByRole('alert')).textContent).toContain('enlace inválido o vencido');
  });
});
