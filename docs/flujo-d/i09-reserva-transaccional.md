# I09 · Reserva transaccional y adjuntos protegidos (flujo D)

**Estado:** implementado en la rama `feature/reserva-i09`; pendiente de revisión.
**Requisitos:** RF-10, RF-11, RF-13 · **Reglas:** RN-03, RN-04, RN-05, RN-07, D05, D06, D07, RN-16, RN-17 · **Depende de:** I04 (integrada), I06 (integrada), I08 (aún no publicada).

## Alcance de la entrega

| Pieza | Archivo | Qué resuelve |
|---|---|---|
| Transacción | `models/reserva/repo_postgres.go` | `pg_advisory_xact_lock` por escolar, conteo de cuota dentro de la transacción, inserción con `RETURNING id` y traducción de errores de integridad |
| Solapamiento | migración `007` | `EXCLUDE USING gist (escolar_id WITH =, tstzrange(inicio, fin, '[)') WITH &&)` |
| Idempotencia | migración `007` + modelo | índice único `(escolar_id, clave_idempotencia)`, reintento con la misma clave y mismos datos devuelve la misma reserva; con otros datos responde 409 |
| Cuota D07 | modelo + transacción | máximo 2 reservas futuras activas y 1 sesión por curso por semana de Lima (lunes 00:00) |
| Adjuntos | `models/reserva/adjuntos.go` | validación por contenido, cuarentena, análisis, estados y puerto de almacenamiento |
| Almacén | `internal/adapters/almacen_adjuntos.go` | archivos con nombre opaco de 32 hex, permisos 0600, directorio 0700 |
| Enlaces privados | `controllers/reserva/controlador.go` | descarga autenticada solo para participantes y solo si el adjunto está `validado` |
| Vista | `frontend/src/views/reserva/FormularioReserva.jsx` | sube los archivos después de confirmar y muestra el estado de cada uno |

## Cómo funciona la reserva transaccional

1. El servicio valida datos, ventana de 48 horas y horizonte.
2. Si viene `Idempotency-Key`, busca la reserva previa: mismos datos → la devuelve (200); otros datos → `idempotencia_conflicto` (409).
3. El repositorio abre una transacción y toma `pg_advisory_xact_lock(hashtext(escolar_id))`, así dos peticiones del mismo escolar se serializan.
4. Dentro de la transacción: slot libre, reservas futuras, sesiones del mismo curso en la semana, inserción.
5. La base sigue siendo la última defensa: `reservas_slot_uq` (23505) y `reservas_sin_solapamiento` (23P01) se traducen a `slot_ocupado` y `solapamiento`.

## Adjuntos: cuarentena y análisis

```text
POST /api/reserva (JSON)      -> reserva confirmada (sin archivos)
POST /api/reserva/{id}/adjuntos (multipart)
      validar nombre y tamaño
      detectar tipo por contenido (PDF, PNG, JPG, TXT, DOCX)
      guardar en almacén privado con nombre opaco
      analizar: validado | rechazado | en_cuarentena (si el análisis no está disponible)
GET  /api/reserva/{id}/adjuntos            -> metadatos y estado
GET  /api/reserva/{id}/adjuntos/{adjuntoId} -> contenido solo si validado
```

- La reserva **no depende** del adjunto: si el análisis falla o el archivo se rechaza, la reserva sigue confirmada.
- Un archivo rechazado no conserva el contenido; queda el metadato con el motivo (evita almacenar material no permitido).
- Un adjunto en cuarentena no se descarga: `409 adjunto_en_cuarentena`.
- Permisos: sube solo el escolar dueño; descargan el escolar, el tutor asignado y coordinación.
- Límites: 5 adjuntos por reserva, 10 MB por archivo; el tipo se decide por contenido, no por lo que declara el cliente.

## Evidencia

Capturas y resultados del recorrido en [evidencia-i09.md](evidencia-i09.md): formulario con adjunto,
reserva confirmada con descarga validada, cuarentena sin análisis y pruebas HTTP de idempotencia.

## Pruebas

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
# con base real (crea una base aislada por paquete y aplica 001-007):
TEST_DATABASE_URL=postgres://... go test -count=1 ./internal/models/reserva
cd frontend && npm test && npm run build
```

- Modelo: 48 h, horizonte, semana de Lima, cuota, solapamiento, idempotencia, adjuntos (cuarentena, rechazo, límite, tipo, permisos).
- Controlador: 201/200 por idempotencia, 409 de cuota y conflicto, multipart, 413, 422, descarga con permisos y cuarentena.
- Integración con PostgreSQL: 10 goroutines por el mismo slot (gana una), 10 con la misma clave (una sola reserva), solapamiento concurrente y cuota concurrente.

## Fuera de alcance (declarado)

- **I06:** la verificación "sin autorización vigente no se reserva" la publica Víctor; el punto de integración queda pendiente para cuando exista su API.
- **I08:** la oferta real por semana; hoy se consume la oferta sintética por `PuertoOferta`.
- **I16:** cancelación y reprogramación.
- Antivirus real: el análisis del MVP es la firma EICAR y el tipo detectado; el servicio se sustituye por un adaptador sin cambiar el modelo.
