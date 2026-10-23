CREATE TABLE IF NOT EXISTS autorizaciones_apoderado (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vinculacion_id UUID NOT NULL REFERENCES vinculaciones_apoderado(id) ON DELETE CASCADE,
    apoderado_id   UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    escolar_id     UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    estado         TEXT NOT NULL CHECK (estado IN ('autorizado', 'no_autorizado')),
    creada_en      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT autorizacion_partes_distintas
        CHECK (apoderado_id <> escolar_id)
);

-- Cada fila es un cambio de estado. El vigente es la fila más reciente.
CREATE INDEX IF NOT EXISTS autorizacion_escolar_idx
    ON autorizaciones_apoderado (escolar_id, creada_en DESC, id DESC);

CREATE INDEX IF NOT EXISTS autorizacion_vinculacion_idx
    ON autorizaciones_apoderado (vinculacion_id, creada_en DESC, id DESC);

-- Los vínculos aceptados antes de esta migración quedan autorizados, que es el
-- mismo estado inicial de una aceptación nueva.
INSERT INTO autorizaciones_apoderado (vinculacion_id, apoderado_id, escolar_id, estado, creada_en)
SELECT v.id, v.apoderado_id, v.escolar_id, 'autorizado', COALESCE(v.aceptada_en, v.creada_en)
FROM vinculaciones_apoderado v
WHERE v.estado = 'aceptada'
  AND NOT EXISTS (
      SELECT 1 FROM autorizaciones_apoderado a WHERE a.vinculacion_id = v.id
  );
