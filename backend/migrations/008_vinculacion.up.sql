CREATE TABLE IF NOT EXISTS vinculaciones_apoderado (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    apoderado_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    escolar_id   UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    estado       TEXT NOT NULL CHECK (estado IN ('pendiente', 'aceptada')),
    token_hash   BYTEA UNIQUE,
    expira_en    TIMESTAMPTZ NOT NULL,
    creada_en    TIMESTAMPTZ NOT NULL DEFAULT now(),
    aceptada_en  TIMESTAMPTZ,

    CONSTRAINT vinculacion_partes_distintas
        CHECK (apoderado_id <> escolar_id),

    CONSTRAINT vinculacion_estado_fechas
        CHECK (
            (estado = 'pendiente' AND aceptada_en IS NULL)
            OR (estado = 'aceptada' AND aceptada_en IS NOT NULL)
        )
);

-- Un escolar solo puede tener un apoderado aceptado.
CREATE UNIQUE INDEX IF NOT EXISTS vinculacion_escolar_aceptada_uq
    ON vinculaciones_apoderado (escolar_id)
    WHERE estado = 'aceptada';

-- Acelera la búsqueda de solicitudes pendientes y el listado por apoderado.
CREATE INDEX IF NOT EXISTS vinculacion_escolar_pendiente_idx
    ON vinculaciones_apoderado (escolar_id)
    WHERE estado = 'pendiente';

CREATE INDEX IF NOT EXISTS vinculacion_apoderado_idx
    ON vinculaciones_apoderado (apoderado_id, creada_en DESC);
