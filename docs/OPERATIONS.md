# HorneroDB — Runbook Operativo (v1.0 GA)

Guía de operación para desplegar, observar, respaldar y actualizar una instancia
de HorneroDB v1.0 en producción.

- **API base:** `http://localhost:8080` (prefijo `/api/v1`)
- **Especificación completa:** [`docs/openapi.yaml`](./openapi.yaml)
- **Seguridad y superficie de ataque:** [`docs/SECURITY.md`](./SECURITY.md)
- **Scripts de backup/restore:** [`scripts/backup.sh`](../scripts/backup.sh), [`scripts/restore.sh`](../scripts/restore.sh), [`scripts/backup.ps1`](../scripts/backup.ps1)

---

## 1. Checklist pre-producción

El servidor **falla rápido** en producción: si falta `JWT_SECRET`, si la
contraseña de la base es la de fábrica o si `DB_SSLMODE=disable`, el proceso
no arranca (ver `internal/config/config.go`). Eso convierte este checklist en
una precondición, no en una recomendación.

### 1.1 Variables obligatorias

| Variable | Valor requerido | Por qué |
|---|---|---|
| `HORNERO_ENV` | `production` | Activa el modo producción: logs JSON, UI embebida, validación fail-fast, cookies `Secure`. También lo activan `ENV=production` y `NODE_ENV=production`. |
| `JWT_SECRET` | ≥ 32 caracteres, nunca el valor por defecto | Firma las sesiones y los tokens MCP. **Rotarlo invalida todas las sesiones.** Generar con `openssl rand -base64 32`. |
| `DB_PASSWORD` | Contraseña fuerte, distinta de `postgres` | Acceso a PostgreSQL. Con la de fábrica el arranque falla en producción. |
| `DB_SSLMODE` | `require` (o `verify-full`) | Cifrado del canal a la base. `disable` está rechazado en producción. |
| `HORNERO_ADMIN_URL` | `https://admin.mydomain.com` | **Es una frontera de seguridad**: define el origen permitido por CORS *y* por la protección CSRF de las escrituras. Si queda apuntando a `localhost` mientras la UI se sirve desde otro origen, **todas** las escrituras fallan. |
| `TRUSTED_PROXIES` | IPs/CIDR del proxy (ej. `172.16.0.0/12,10.0.0.1`) | Sin esto el servidor **no confía en nadie** y descarta `X-Forwarded-For`/`X-Forwarded-Proto`; con esto, confía solo en esos rangos. Vacío = no confiar en ningún proxy (correcto si el servidor está expuesto directamente). |
| `HORNERO_SETUP_TOKEN` | `openssl rand -base64 32` | Secreto de bootstrap: debe presentarse en `X-Setup-Token` para completar el **primer** setup. Se puede rotar una vez que la instancia tenga administrador. |

### 1.2 Variables recomendadas

| Variable | Default | Nota |
|---|---|---|
| `SERVER_PORT` | `8080` | Puerto interno del proceso. |
| `VITE_SERVER_PUBLIC_URL` | `http://localhost:5173` | URL pública de la instancia; se usa en el callback OIDC y en la URL de retorno del login. **En producción debe ser el dominio real.** |
| `CORS_ORIGINS` | (vacío → solo `HORNERO_ADMIN_URL`) | Lista separada de orígenes permitidos si la UI y la API se sirven desde dominios distintos. |
| `POCKETID_ISSUER_URL` / `POCKETID_PUBLIC_URL` | — | Proveedor OIDC (PocketID). `POCKETID_REDIRECT_URL` por defecto: `http://localhost:8080/api/v1/auth/oidc/callback`. |
| `HORNERO_SECURE_COOKIES` | auto | En producción la cookie de sesión ya sale con `Secure`. Fijalo en `true` sólo si exponés el servidor por HTTPS en otro host. |

### 1.3 Escape hatches (sólo redes internas)

`DB_ALLOW_INSECURE_SSL` y `DB_ALLOW_INSECURE_DB` permiten `DB_SSLMODE=disable` y
la contraseña por defecto. Vienen en `true` en el `docker-compose.yml` incluido
porque el PostgreSQL del bundle vive en la red interna de compose.

**Nunca los actives en un despliegue expuesto a Internet.**

### 1.4 Verificación del arranque

```bash
docker compose up -d
docker compose logs -f hornerodb-server

# debe responder 200 con {"status":"ok"}
curl -fsS http://localhost:8080/health

# debe responder 200 con {"status":"ready"} (verifica PostgreSQL)
curl -fsS http://localhost:8080/ready
```

Después completá el setup inicial con el token:

```bash
curl -X POST https://api.mydomain.com/api/v1/setup/complete \
  -H "X-Setup-Token: $HORNERO_SETUP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"instance_name":"Acme Corp","contact_email":"ops@acme.com"}'
```

---

## 2. Reverse proxy con TLS

El servidor **no** termina TLS: habla HTTP plano y espera que el proxy lo
haga. Esos dos headers son la frontera de confianza de la IP del cliente, así
que **siempre** hay que declarar qué proxies son de confianza.

### 2.1 Caddy

Caddy emite el certificado automáticamente y reenvía los headers por defecto.

```caddyfile
acme.mydomain.com {
    encode gzip

    # El healthcheck no necesita pasar por rate limiting ni por autenticación.
    @health path /health /ready
    handle @health {
        reverse_proxy 127.0.0.1:8080
    }

    handle {
        reverse_proxy 127.0.0.1:8080 {
            # Caddy ya envía X-Forwarded-For / X-Forwarded-Proto / X-Forwarded-Host.
            header_up X-Real-IP {remote_host}
        }
    }

    log {
        output stdout
        format json
    }
}
```

### 2.2 Nginx

Nginx **no** reenvía `X-Forwarded-For` de forma fiable por defecto: hay que
añadirlo explícitamente, o el servidor verá la IP del proxy en todos los logs
y en el rate limiting.

```nginx
server {
    listen 443 ssl http2;
    server_name acme.mydomain.com;

    ssl_certificate     /etc/letsencrypt/live/acme.mydomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/acme.mydomain.com/privkey.pem;

    # Cabeceras de seguridad (el servidor ya envía sus propias, esto reinforce).
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    client_max_body_size 25m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        # ── Esencial: el servidor usa estos headers para el IP real y el esquema ──
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host  $host;

        # Webhooks y MCP SSE necesitan streaming sin buffering.
        proxy_buffering off;
        proxy_read_timeout 300s;
    }

    location = /health {
        access_log off;
        proxy_pass http://127.0.0.1:8080/health;
    }

    access_log /var/log/nginx/hornerodb.access.log combined;
    error_log  /var/log/nginx/hornerodb.error.log warn;
}
```

### 2.3 `TRUSTED_PROXIES` coherente con el proxy

```bash
# Nginx/Caddy en el mismo host, escuchando en loopback:
TRUSTED_PROXIES=127.0.0.1/32,::1/128

# Proxy en otra máquina de la red de compose:
TRUSTED_PROXIES=172.16.0.0/12

# Varios rangos separados por coma:
TRUSTED_PROXIES=10.0.0.1,172.16.0.0/12,192.168.1.0/24
```

Reglas prácticas:

- **Nunca** `0.0.0.0/0`: permitiría falsificar `X-Forwarded-For` desde
  cualquier cliente y romper el rate limiting por IP y el registro de auditoría.
- Si el servidor está **expuesto directamente** (sin proxy), dejá
  `TRUSTED_PROXIES` vacío: no se confía en ningún `X-Forwarded-*`.
- El proxy debe **sobrescribir** (`proxy_set_header`), no reenviar el valor
  original sin validar: un cliente puede mandar `X-Forwarded-For` falsificado.

---

## 3. Healthchecks y logs

### 3.1 Liveness vs readiness

| Endpoint | Consulta la base | Uso | Código de error |
|---|---|---|---|
| `GET /health` | No | **Liveness**: ¿el proceso responde? Si falla, hay que **reiniciar** el contenedor. | `200` siempre mientras el proceso viva |
| `GET /ready` | Sí (ping con timeout de 2 s) | **Readiness**: ¿puede atender tráfico? Si falla, hay que **quitar la instancia del balanceador**, no reiniciarla. | `503` con `{"status":"unavailable"}` |

Configuración en Docker Compose (ya incluida en `docker-compose.yml`):

```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -qO- http://localhost:${SERVER_PORT:-8080}/health || exit 1"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 10s
```

En Kubernetes:

```yaml
livenessProbe:
  httpGet: { path: /health, port: 8080 }
  initialDelaySeconds: 10
  periodSeconds: 30
readinessProbe:
  httpGet: { path: /ready, port: 8080 }
  initialDelaySeconds: 5
  periodSeconds: 10
```

> En v1.0 la instancia es **single-node** (§5.2): usar el mismo probe para
> liveness y readiness es aceptable. Con réplicas, `liveness` debe seguir
> apuntando a `/health` para no reiniciar un nodo sano durante una caída
> transitoria de la base.

### 3.2 Logs estructurados

En producción el servidor usa `slog` con `JSONHandler`: **una línea JSON por
evento**, lista para ingesting en Loki, Datadog, CloudWatch o `jq`.

```json
{"time":"2026-11-22T14:11:23.481Z","level":"INFO","msg":"request processed","status":200,"method":"GET","path":"/api/v1/workspaces","ip":"203.0.113.10","latency":12043000,"user_agent":"curl/8.5.0","user_id":"8f2c…","request_id":"6b1d…"}
```

Campos por request:

| Campo | Significado |
|---|---|
| `status` | Código HTTP de la respuesta. |
| `method`, `path` | Método y ruta. **El query string nunca se loguea**: suele llevar tokens, códigos y secretos. |
| `ip` | IP del cliente, ya resuelta a través de `X-Forwarded-For` **sólo** si el proxy está en `TRUSTED_PROXIES`. |
| `latency` | Duración en nanoseconds. |
| `user_agent` | User-Agent del cliente. |
| `user_id` | Identificador del usuario autenticado (presente sólo si lo hay). |
| `request_id` | `X-Request-ID` entrante o uno generado. Es el identificador para correlacionar con el proxy. |

Eventos de dominio relevantes (además de los de request):

| `msg` | Cuándo |
|---|---|
| `record created` / `record updated` / `record deleted` | Mutaciones de datos. |
| `role created` / `role updated` / `role deleted` | Cambios de permisos. |
| `API key created` / `API key rotated` / `API key deleted` | Ciclo de vida de credenciales. |
| `webhook created` / `webhook delivery failed` | Suscripciones y entregas fallidas. |
| `initial setup completed` | Bootstrap de la instancia. |
| `workspace schema exported` / `imported` | Exportaciones e importaciones. |

Consultas útiles:

```bash
# Errores 4xx/5xx de los últimos minutos
docker compose logs hornerodb-server --since 10m \
  | jq -c 'select(.level=="ERROR")'

# Latencia alta
docker compose logs hornerodb-server --since 1h \
  | jq -c 'select(.latency > 1000000000) | {path, latency}'

# Correlacionar una petición con el proxy
docker compose logs hornerodb-server | jq -c 'select(.request_id=="6b1d…")'
```

### 3.3 Auditoría

Las mutaciones sensibles (API keys, roles, webhooks, credenciales de
recuperación) se registran además en la tabla `_hornero_audit_log`:

```bash
curl -fsS -H "Authorization: Bearer $TOKEN" \
  "https://api.mydomain.com/api/v1/workspaces/$WS/audit?action=api_key.created&limit=50"
```

Las claves que parecen credenciales (`secret`, `password`, `token`, `key_hash`)
se eliminan del `metadata` antes de persistirlo: **la auditoría nunca guarda
un secreto**.

---

## 4. Backup y Restore

### 4.1 Qué se respalda

| Componente | Ubicación | Método |
|---|---|---|
| PostgreSQL (datos + metadatos) | contenedor `postgres` | `pg_dump --format=custom` |
| PocketID (usuarios, OIDC) | `./data/pocket-id.db` (bind mount a `/app/data`) | `sqlite3 .backup`, o copia atómica si `sqlite3` no está instalado |

El dump en formato *custom* de PostgreSQL es comprimido, selectivo y
restaurable con `pg_restore --clean --if-exists`.

### 4.2 Backup

> En Linux/macOS, la primera vez: `chmod +x scripts/backup.sh scripts/restore.sh`.

```bash
# Desde la raíz del repositorio
./scripts/backup.sh

# Con opciones explícitas
BACKUP_DIR=/mnt/backups/hornerodb RETENTION_DAYS=30 ./scripts/backup.sh
./scripts/backup.sh --dir /mnt/backups/hornerodb --retention 30
```

El script:

1. Vuelca PostgreSQL (`pg_dump --format=custom`) usando `docker compose exec -T postgres`
   o el `pg_dump` local, según `BACKUP_MODE=auto|docker|direct`.
2. Copia `data/pocket-id.db` de forma consistente.
3. Calcula los checksums **SHA-256** y los guarda en `SHA256SUMS`.
4. Comprime todo en `backups/hornerodb-backup-<UTC timestamp>.tar.gz` con
   permisos `600`.
5. Aplica la retención: borra los `.tar.gz` con más de `RETENTION_DAYS`
   (default **14**) días.

En Windows / PowerShell:

```powershell
.\scripts\backup.ps1
.\scripts\backup.ps1 -BackupDir D:\backups\hornerodb -RetentionDays 30
```

### 4.3 Restore

```bash
./scripts/restore.sh backups/hornerodb-backup-20261122T141123Z.tar.gz
# o sin interacción (para cron / runbooks):
./scripts/restore.sh backups/hornerodb-backup-20261122T141123Z.tar.gz --yes
# parando la aplicación durante el restore:
./scripts/restore.sh backups/…tar.gz --yes --stop-services
```

El script:

1. Extrae el archivo en un directorio temporal.
2. **Verifica los checksums SHA-256** de cada componente. Si uno no coincide,
   aborta **sin tocar nada**: no se restaura un backup corrupto.
3. Pide confirmación interactiva (o acepta `--yes`).
4. Restaura PostgreSQL con `pg_restore --clean --if-exists` (borra los objetos
   existentes y los recrea; `--if-exists` evita errores cuando el objeto no
   estaba).
5. Guarda la base actual de PocketID como
   `data/pocket-id.db.pre-restore-<timestamp>` y luego la reemplaza de forma
   atómica.

### 4.4 Verificación periódica

Un backup que nunca se restauró **no es un backup**. Recomendado:

- **Diario**: `backup.sh` por cron.
- **Semanal**: restauración en un entorno aislado y comparación de conteos.
- **Mensual**: drill de recuperación completo con cronómetro.

```bash
# 0 2 * * *  backup diario
0 2 * * * cd /opt/hornerodb && ./scripts/backup.sh >> /var/log/hornerodb-backup.log 2>&1
```

Checklist del drill:

```bash
# 1. En una instancia aislada, restaurar el backup más reciente
./scripts/restore.sh backups/hornerodb-backup-<más reciente>.tar.gz --yes

# 2. La API tiene que volver a estar sana
curl -fsS https://restore.mydomain.com/ready

# 3. Comparar el número de workspaces, tablas y registros
curl -fsS -H "Authorization: Bearer $TOKEN" \
  "https://restore.mydomain.com/api/v1/workspaces" | jq '.meta.count'

# 4. Verificar que un registro crítico es intacto
curl -fsS -H "Authorization: Bearer $TOKEN" \
  "https://restore.mydomain.com/api/v1/workspaces/$WS/data/clientes?limit=1" | jq '.data[0]'
```

Anotar siempre la **duración** del restore: es el dato que define el RPO/RTO
real de la organización.

### 4.5 Restaurar sólo PostgreSQL (PocketID intacto)

```bash
./scripts/restore.sh backups/hornerodb-backup-<ts>.tar.gz --yes --no-pocketid
```

---

## 5. Upgrade sin pérdida de datos

### 5.1 Procedimiento

```bash
# 1. Backup verificado (OBLIGATORIO antes de tocar nada)
./scripts/backup.sh
# Verificar que el archivo existe y que el checksum es correcto
tar -xzf backups/hornerodb-backup-<ts>.tar.gz -O SHA256SUMS | sha256sum -c

# 2. Traer la nueva versión
git fetch --tags && git checkout <tag-o-commit-nuevo>

# 3. Reconstruir la imagen (las migraciones se aplican en el arranque)
docker compose build hornerodb-server

# 4. Reiniciar. La API corre migraciones incrementales al arrancar.
docker compose up -d hornerodb-server

# 5. Verificar
curl -fsS https://api.mydomain.com/ready
docker compose logs --since 5m hornerodb-server | grep -iE "migrat|error"
```

Claves del procedimiento:

- **Las migraciones son automáticas e incrementales**: se ejecutan dentro de
  `database.RunMigrations()` durante el arranque, antes de aceptar tráfico.
  `docker compose up -d` es suficiente; no hay paso manual de migración.
- **Son sólo aditivas**: la GA-0/GA-1 endureció autenticación, rate limiting,
  CSRF, CSP y la capa de permisos sin romper el modelo de datos. Las
  migraciones que agregan columnas usan valores por defecto, de modo que las
  filas existentes se leen sin backfill.
- **El esquema físico vive junto a los metadatos**: cada tabla de negocio es
  `data_<workspace_id>_<slug>` y sus columnas se agregan/renombran en la misma
  transacción que la metadata. Un upgrade nunca deja el esquema a medias.
- **Rollback**: `git checkout <tag-anterior> && docker compose build &&
  docker compose up -d`. Si la versión nueva migration ya se aplicó, este
  rollback **no** es seguro: preferí restaurar el backup del paso 1.
- **Verificá el esquema tras el upgrade**: las columnas `select` usan el
  formato canónico `meta.choices` (`[{ "value": …, "label": … }]`); el
  formato legacy `meta.options` sigue leyéndose pero no debe usarse en
  plantillas nuevas.

### 5.2 Límites explícitos de v1.0

Lo que **no** está soportado en v1.0 GA:

| Límite | Detalle | Planificado |
|---|---|---|
| **Single-node** | Una sola réplica de la aplicación. El rate limiting en memoria y el dispatcher de webhooks en proceso asumen una sola instancia: al escalar horizontalmente sin deduplicar, los webhooks se entregarían más de una vez. | v1.1 |
| **Redis multi-réplica** | No hay backend Redis para el rate limiting distribuido ni para la cola de webhooks. El outbox de webhooks es una tabla de PostgreSQL, pero el worker corre en cada réplica. | v1.1 |
| **PostgreSQL RLS nativa** | La seguridad por fila (RLS) la implementa la capa de aplicación filtrando por `created_by` en la capa de datos, **no** `ROW LEVEL SECURITY` de PostgreSQL. Un acceso directo a la base no está aislado por workspace. | v1.1 |
| **Alta disponibilidad de la base** | Se asume una única instancia de PostgreSQL. No hay PgBouncer, ni particionado, ni replicación read-only gestionada. | v1.1 |
| **Esquema entre versiones mayores** | No hay migraciones *down*. El rollback de un upgrade ya migrado se hace restaurando un backup. | — |
| **Backups incrementales** | El backup es completo. No hay WAL archiving (`pg_basebackup`) ni PITR. | v1.1 |

Operativamente, esto significa: **una instancia, un PostgreSQL, un backup
completo diario y un restore probado**.

---

## 6. Operación diaria — resumen

| Tarea | Frecuencia | Comando / acción |
|---|---|---|
| Backup completo | Diario (cron) | `./scripts/backup.sh` |
| Verificación de salud | Automática | `/health` y `/ready` en el healthcheck de compose |
| Revisión del trail de auditoría | Semanal | `GET /api/v1/workspaces/{id}/audit` |
| Limpieza de API keys caducadas | Mensual | `GET /api/v1/workspaces/{id}/keys` → revisar `expires_at` |
| Revisión de entregas de webhooks fallidas | Semanal | `GET /api/v1/workspaces/{id}/webhooks/events?status=failed` → `POST …/retry` |
| Drill de restore | Mensual | §4.4 |
| Rotación de `JWT_SECRET` | Semestral | Invalida todas las sesiones: avisar a los usuarios antes. |
| Rotación de claves de API | Cada 90 días | `POST /api/v1/workspaces/{id}/keys/{key_id}/rotate` |

---

## 7. Troubleshooting

| Síntoma | Causa probable | Acción |
|---|---|---|
| `401` en todas las escrituras desde el navegador | `HORNERO_ADMIN_URL` no coincide con el origen real de la UI (CORS/CSRF) | Corregir `HORNERO_ADMIN_URL` y reiniciar. |
| Todos los clientes ven la misma IP en los logs | Falta `TRUSTED_PROXIES` o el proxy no reenvía `X-Forwarded-For` | Configurar ambos (proxy + variable). |
| La cookie de sesión no se guarda | Cookie sin `Secure` detrás de un proxy HTTP | Publicar por HTTPS; `HORNERO_SECURE_COOKIES=true` si el TLS termina fuera. |
| `403` en el setup inicial | `HORNERO_SETUP_TOKEN` incorrecto o ausente | Enviar el token en `X-Setup-Token`. |
| El servidor no arranca | Validación fail-fast de producción | Leer el error: falta `JWT_SECRET`, `DB_PASSWORD` por defecto o `DB_SSLMODE=disable`. |
| Rate limiting inesperado | `TRUSTED_PROXIES` demasiado amplio (spoofing de `X-Forwarded-For`) | Acotar la lista a los rangos reales del proxy. |
| `ERR_VALIDATION` al escribir en un `select` | El valor no está en `meta.choices` | Usar el `value` de una opción, no el `label`. |
| La relación aparece vacía | `display_column` inexistente o el rol no puede leer la tabla destino | Revisar `meta.target_table` / `meta.display_column` y los permisos de lectura del rol. |
