// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
} from 'react-router-dom';
import RutaProtegidaConRetorno from './RutaProtegidaConRetorno.jsx';
import { useSesion } from '../controllers/acceso/useSesion.js';

vi.mock('../controllers/acceso/useSesion.js', () => ({
  useSesion: vi.fn(),
}));

function Destino() {
  const ubicacion = useLocation();
  return <p>{`${ubicacion.pathname}${ubicacion.search}`}</p>;
}

function renderizar(entrada = '/vinculacion/aceptar?token=abc') {
  return render(
    <MemoryRouter initialEntries={[entrada]}>
      <Routes>
        <Route
          path="/vinculacion/aceptar"
          element={
            <RutaProtegidaConRetorno roles={['escolar']}>
              <p>contenido privado</p>
            </RutaProtegidaConRetorno>
          }
        />
        <Route path="/ingreso" element={<Destino />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('RutaProtegidaConRetorno', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('conserva el destino completo, incluido el token, al redirigir al login', async () => {
    useSesion.mockReturnValue({ usuario: null, cargando: false });

    renderizar();

    expect(
      await screen.findByText('/ingreso?next=%2Fvinculacion%2Faceptar%3Ftoken%3Dabc')
    ).toBeTruthy();
    expect(screen.queryByText('contenido privado')).toBeNull();
  });

  it('delega la autorización en RutaProtegida cuando hay sesión', () => {
    useSesion.mockReturnValue({
      usuario: { rol: 'escolar' },
      cargando: false,
    });

    renderizar();

    expect(screen.getByText('contenido privado')).toBeTruthy();
  });
});
