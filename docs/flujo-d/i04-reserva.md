# I04 · Reserva base con formulario (flujo D)

**Estado:** implementado en la rama `feature/reserva-i04`; pendiente de revisión de Víctor y de merge.
**Requisitos:** RF-10, RF-11 · **Reglas:** RN-03, RN-04, RN-07, D05, D06 · **Depende de:** E02.

## Alcance de la entrega

| Capa | Archivos |
|---|---|
| Modelo | `backend/internal/models/reserva/`: `reserva.go` (entidades y reglas), `oferta.go` (puerto del flujo C + oferta sintética), `servicio.go`, `repositorio.go`, `repo_memoria.go`, `repo_postgres.go` |
| Controlador | `backend/internal/controllers/reserva/controlador.go` |
| Vista | `backend/internal/views/reserva/presentador.go` y `frontend/src/views/reserva/FormularioReserva.jsx` |
| Cliente | `frontend/src/controllers/reserva/apiReserva.js` + `frontend/src/models/reserva/validadores.js` |
| Datos | `backend/migrations/004_reserva.{up,down}.sql`, `backend/fixtures/reserva.json` |
| Contrato | `docs/openapi.yaml` (etiqueta `D-reserva`) |
| Pruebas | `backend/internal/models/reserva/reserva_test.go`, `backend/internal/controllers/reserva/controlador_test.go`, `frontend/src/controllers/reserva/apiReserva.test.js` |

## Reglas que se aplican en el modelo

- **RN-03 (48 horas exactas):** `inicio >= ahora + 48 h`, con la hora del servidor. Una sesión a
  48 h justas se acepta; un nanosegundo menos se rechaza.
- **D06 (sesiones de 60 minutos y horizonte de 28 días).** El horizonte se valida en el mismo punto.
- **RN-07:** tema 10–200, objetivo 20–1000, grado obligatorio, adjuntos opcionales (máximo 5, 10 MB
  cada uno, sin rutas en el nombre).
- **D05/RN-06:** la reserva válida queda `confirmada` de inmediato; no hay segunda aceptación.
- La **vista** no decide nada: recibe `disponible` y `motivo` por horario.
- El **escolar sale de la cookie de sesión**, nunca del cuerpo de la petición.

## Decisiones de diseño

1. **Puerto de oferta.** `PuertoOferta` aísla al flujo D del flujo C. Hoy lo implementa
   `OfertaSintetica` (horarios calculados al arrancar: cuatro válidos, uno dentro de 48 h y uno
   fuera del horizonte, para demostrar la regla). Cuando Mateo publique I03/I08 se cambia solo esa
   implementación.
2. **Identificadores externos sin clave foránea.** `tutor_id` y `slot_id` son `TEXT`: pertenecen
   al flujo C y no se puede escribir en sus tablas. `escolar_id` sí es `UUID` con `REFERENCES
   usuarios(id)` porque el usuario es modelo compartido.
3. **Copia del horario.** La reserva guarda `tutor_nombre`, `curso`, `inicio` y `fin` como
   fotografía del horario reservado, para que el historial no cambie si la oferta se edita.
4. **Hora de Lima sin base de zonas.** `time.FixedZone("America/Lima", -5h)` porque el Perú no
   aplica horario de verano; se evita depender de la base de zonas del sistema operativo.
5. **Adjuntos solo con metadatos.** Subir archivos, cuarentena y enlaces privados son I09. El
   formulario envía `{nombre, tipo, tamano_bytes}` y la tabla `reserva_adjuntos` guarda eso.
6. **Unicidad por slot desde ya.** `reservas_slot_uq` protege el dato aunque la prueba de carreras
   llegue en I09; el segundo intento responde `409 slot_ocupado`.
7. **Ruta registrada con y sin barra final.** `POST /api/reserva` y `/api/reserva/` comparten
   manejador: sin el patrón exacto, el `ServeMux` responde 301 y el navegador convertiría el POST
   en GET.

## Evidencia del recorrido

Capturas del recorrido completo (ingreso, formulario con motivos, confirmación y horario ocupado) en
[evidencia-i04.md](evidencia-i04.md), con la tabla de horarios y el texto de confirmación.

## Pruebas

```bash
cd backend && gofmt -l . && go vet ./... && go test -race ./...
cd frontend && npm install && npm test && npm run build
```

Casos cubiertos: 48 h exactas y borde inferior, horizonte de 28 días, horario vencido, validación por
campo, adjunto normalizado, slot inexistente, doble reserva del mismo horario, motivo de rechazo en el
listado, consulta solo por su dueño y recorrido HTTP completo (401/403/201/404/409/422).

## Demostración manual

1. `SEED_SYNTHETIC=true` con `SEED_PASSWORD`, arrancar la API (`go run ./cmd/server`).
2. Entrar como `apoderado.prueba@example.test`: la semilla ya creó el vínculo aceptado con
   `escolar.prueba@example.test`. En `/panel_escolares` se ve la autorización vigente y su historial.
3. Entrar como `escolar.prueba@example.test` y abrir `/reservar`: el horario a menos de 48 h aparece
   deshabilitado con su motivo.
4. Enviar tema, objetivo y grado; la reserva se confirma y muestra la hora en America/Lima.
5. Repetir con el mismo horario: `409`.

## Integración con I06

`POST /api/reserva` consulta el puerto `PuertoAutorizacion` del flujo A. Si el escolar no tiene
un apoderado vinculado o su apoderado revocó el permiso, responde `403 sin_autorizacion`. Las
reservas ya confirmadas no se modifican: la comprobación ocurre únicamente al crear una nueva.

## Fuera de alcance (declarado, no inventado)

- **I16:** cancelar y reprogramar con historial.
- La oferta es sintética: no hay catálogo de cursos ni tutores reales hasta I03/I07/I08.

La concurrencia, idempotencia, cuotas y adjuntos del flujo D se documentan en
[I09](i09-reserva-transaccional.md).
