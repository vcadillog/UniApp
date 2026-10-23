// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import VincularEscolar from './VincularEscolar.jsx';
import { solicitarVinculacion } from '../../controllers/apoderado/apiApoderado.js';

vi.mock('../../controllers/apoderado/apiApoderado.js', () => ({
  solicitarVinculacion: vi.fn(),
}));

function renderizar() {
  return render(
    <MemoryRouter>
      <VincularEscolar />
    </MemoryRouter>
  );
}

describe('I06: vincular escolar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('envía el correo del escolar', async () => {
    solicitarVinculacion.mockResolvedValue({ vinculacion: { id: 'v-1' } });

    renderizar();

    expect(
      await screen.findByRole('heading', { name: 'Vincular a un escolar' })
    ).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Ir al panel de escolares' })).toBeTruthy();

    fireEvent.change(screen.getByLabelText('Correo del escolar'), {
      target: { value: 'escolar@example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Enviar solicitud' }));

    expect(await screen.findByText(/Solicitud enviada/)).toBeTruthy();
    expect(solicitarVinculacion).toHaveBeenCalledWith('escolar@example.com');
  });

  it('muestra el error de validación del backend', async () => {
    solicitarVinculacion.mockRejectedValue({
      message: 'Revisa los datos ingresados.',
      campos: { email_escolar: 'el correo es obligatorio' },
    });

    renderizar();

    fireEvent.change(await screen.findByLabelText('Correo del escolar'), {
      target: { value: 'escolar@example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Enviar solicitud' }));

    expect(await screen.findByText('el correo es obligatorio')).toBeTruthy();
    expect(screen.getAllByRole('alert').length).toBeGreaterThan(0);
  });
});
