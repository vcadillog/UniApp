// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import MiApoderado from './MiApoderado.jsx';
import { miApoderado } from '../../controllers/apoderado/apiApoderado.js';

vi.mock('../../controllers/apoderado/apiApoderado.js', () => ({
  miApoderado: vi.fn(),
}));

describe('I06: mi apoderado', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('muestra el vínculo aceptado sin opción de desvincular', async () => {
    miApoderado.mockResolvedValue({
      vinculacion: {
        id: 'v-1',
        estado: 'aceptada',
        apoderado: { nombre_uso: 'Apoderado Prueba', email: 'apoderado@example.com' },
        autorizacion: {
          id: 'a-1',
          estado: 'no_autorizado',
          autorizado: false,
          creada_en: '2026-10-20T12:00:00Z',
        },
      },
    });

    render(
      <MemoryRouter>
        <MiApoderado />
      </MemoryRouter>
    );

    expect(await screen.findByText('Apoderado Prueba')).toBeTruthy();
    expect(screen.getByText(/no se puede desvincular/)).toBeTruthy();
    expect(screen.getByText(/revocó la autorización/)).toBeTruthy();
    expect(screen.getByText(/no se modifican/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: /desvincular/i })).toBeNull();
  });

  it('indica cuando todavía no hay apoderado', async () => {
    miApoderado.mockResolvedValue({ vinculacion: null });

    render(
      <MemoryRouter>
        <MiApoderado />
      </MemoryRouter>
    );

    expect(await screen.findByText('Todavía no tienes un apoderado vinculado.')).toBeTruthy();
  });
});
