#!/usr/bin/env bash
#
# HorneroDB — restore completo (PostgreSQL + PocketID/SQLite).
#
# Restaura un archivo generado por scripts/backup.sh:
#   * verifica los checksums SHA-256 antes de tocar nada
#   * pide confirmación interactiva (o se acepta con --yes)
#   * restaura PostgreSQL con pg_restore --clean --if-exists
#   * restaura data/pocket-id.db guardando antes una copia del estado actual
#
# Uso:
#   ./scripts/restore.sh ARCHIVE.tar.gz [--yes] [--stop-services] [--no-pocketid]
#
# Variables de entorno relevantes:
#   DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
#   POCKETID_DB_FILE   Ruta de la base SQLite de PocketID (por defecto data/pocket-id.db)
#   BACKUP_MODE        auto | docker | direct (por defecto auto)
#   COMPOSE_FILE       Fichero compose alternativo (por defecto docker-compose.yml)
#   COMPOSE_SERVICE    Servicio de PostgreSQL en compose (por defecto postgres)
#   APP_SERVICE        Servicio de la aplicación en compose (por defecto hornerodb-server)
#
set -euo pipefail

# ───────────────────────────── Configuración ─────────────────────────────

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-hornero}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

POCKETID_DB_FILE="${POCKETID_DB_FILE:-data/pocket-id.db}"
BACKUP_MODE="${BACKUP_MODE:-auto}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"
APP_SERVICE="${APP_SERVICE:-hornerodb-server}"

ASSUME_YES=0
STOP_SERVICES=0
INCLUDE_POCKETID=1
ARCHIVE=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ───────────────────────────── Utilidades ─────────────────────────────

log()  { printf '[restore] %s\n' "$*"; }
warn() { printf '[restore] WARNING: %s\n' "$*" >&2; }
die()  { printf '[restore] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
  sed -n '2,28p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit 0
}

sha256_of() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | sed 's/^.*= *//'
  else
    die "no se encontró sha256sum, shasum ni openssl para verificar los checksums"
  fi
}

compose() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    docker compose -f "${COMPOSE_FILE}" "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose -f "${COMPOSE_FILE}" "$@"
  else
    die "docker compose no está disponible; instalalo o usá BACKUP_MODE=direct con pg_restore en el PATH"
  fi
}

detect_mode() {
  if [ "${BACKUP_MODE}" != "auto" ]; then
    echo "${BACKUP_MODE}"
    return
  fi
  if command -v pg_restore >/dev/null 2>&1; then
    echo "direct"
  else
    echo "docker"
  fi
}

# ───────────────────────────── Argumentos ─────────────────────────────

while [ $# -gt 0 ]; do
  case "$1" in
    --yes|-y)         ASSUME_YES=1; shift ;;
    --stop-services)  STOP_SERVICES=1; shift ;;
    --no-pocketid)    INCLUDE_POCKETID=0; shift ;;
    -h|--help)        usage ;;
    -*)               die "opción desconocida: $1 (usá --help)" ;;
    *)                ARCHIVE="$1"; shift ;;
  esac
done

[ -n "${ARCHIVE}" ] || { usage; }
[ -f "${ARCHIVE}" ] || die "no se encontró el archivo de backup: ${ARCHIVE}"

cd "${PROJECT_ROOT}"

MODE="$(detect_mode)"
case "${MODE}" in
  docker)
    command -v docker >/dev/null 2>&1 || die "BACKUP_MODE=docker pero docker no está instalado"
    ;;
  direct)
    command -v pg_restore >/dev/null 2>&1 || die "BACKUP_MODE=direct pero pg_restore no está en el PATH"
    ;;
  *)
    die "BACKUP_MODE inválido: '${MODE}' (valores: auto, docker, direct)"
    ;;
esac

# ───────────────────────────── Extracción ─────────────────────────────

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/hornerodb-restore.XXXXXX")"
cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

log "extrayendo ${ARCHIVE}…"
tar -xzf "${ARCHIVE}" -C "${WORK_DIR}"

[ -f "${WORK_DIR}/postgres.dump" ] || die "el archivo no contiene postgres.dump; no parece un backup de HorneroDB"

# ───────────────────────────── Verificación de checksums ─────────────────────────────

if [ -f "${WORK_DIR}/SHA256SUMS" ]; then
  log "verificando los checksums SHA-256…"
  while read -r expected file_name; do
    [ -n "${expected:-}" ] || continue
    file_name="${file_name#\*}"
    target="${WORK_DIR}/${file_name}"
    [ -f "${target}" ] || die "checksum inválido: falta el archivo '${file_name}' en el backup"
    actual="$(sha256_of "${target}")"
    if [ "${actual}" != "${expected}" ]; then
      die "checksum inválido para '${file_name}': esperado ${expected}, obtenido ${actual}. El backup está corrupto."
    fi
    log "  OK  ${file_name}"
  done < "${WORK_DIR}/SHA256SUMS"
else
  die "el backup no contiene SHA256SUMS: se aborta el restore para no instalar un archivo corrupto"
fi

# ───────────────────────────── Confirmación ─────────────────────────────

if [ "${ASSUME_YES}" -ne 1 ]; then
  printf '\n'
  log "Se va a RESTAURAR y sobreescribir los datos actuales:"
  printf '    PostgreSQL : base %s\n' "${DB_NAME}"
  printf '    PocketID   : %s\n' "${POCKETID_DB_FILE}"
  printf '\n'
  printf '¿Continuar? (escribí "si" para confirmar): '
  read -r answer
  case "${answer}" in
    si|SI|Sí|sí|yes|YES|y) ;;
    *) die "restore cancelado por el operador" ;;
  esac
fi

# ───────────────────────────── Parada opcional de la aplicación ─────────────────────────────

APP_STOPPED=0

stop_app() {
  if [ "${STOP_SERVICES}" -eq 1 ]; then
    log "deteniendo el servicio '${APP_SERVICE}'…"
    compose stop "${APP_SERVICE}" || warn "no se pudo detener '${APP_SERVICE}' (¿no está en compose?)"
    APP_STOPPED=1
  fi
}

start_app() {
  if [ "${APP_STOPPED}" -eq 1 ]; then
    log "volviendo a iniciar el servicio '${APP_SERVICE}'…"
    compose start "${APP_SERVICE}" || warn "no se pudo reiniciar '${APP_SERVICE}'"
  fi
}

stop_app
trap 'start_app; cleanup' EXIT

# ───────────────────────────── Restore de PostgreSQL ─────────────────────────────

log "restaurando PostgreSQL (base '${DB_NAME}')…"

if [ "${MODE}" = "docker" ]; then
  PGPASSWORD="${DB_PASSWORD}" compose exec -T "${COMPOSE_SERVICE}" \
    pg_restore \
      --clean \
      --if-exists \
      --no-owner \
      --no-privileges \
      --dbname="${DB_NAME}" \
      --username="${DB_USER}" \
    < "${WORK_DIR}/postgres.dump"
else
  PGPASSWORD="${DB_PASSWORD}" pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --host="${DB_HOST}" \
    --port="${DB_PORT}" \
    --username="${DB_USER}" \
    --dbname="${DB_NAME}" \
    < "${WORK_DIR}/postgres.dump"
fi

log "PostgreSQL restaurado."

# ───────────────────────────── Restore de PocketID (SQLite) ─────────────────────────────

if [ "${INCLUDE_POCKETID}" -eq 1 ] && [ -f "${WORK_DIR}/pocket-id.db" ]; then
  POCKETID_DIR="$(dirname "${POCKETID_DB_FILE}")"
  mkdir -p "${POCKETID_DIR}"

  if [ -f "${POCKETID_DB_FILE}" ]; then
    SAFETY_COPY="${POCKETID_DB_FILE}.pre-restore-$(date -u +%Y%m%dT%H%M%SZ)"
    log "guardando la base actual en '${SAFETY_COPY}'…"
    cp -p "${POCKETID_DB_FILE}" "${SAFETY_COPY}"
  else
    log "no existía '${POCKETID_DB_FILE}'; se crea con el contenido del backup."
  fi

  # Copia atómica: se escribe en un temporal y se renombra sobre el destino.
  TMP_RESTORE="${POCKETID_DB_FILE}.restore.$$"
  cp "${WORK_DIR}/pocket-id.db" "${TMP_RESTORE}"
  mv -f "${TMP_RESTORE}" "${POCKETID_DB_FILE}"

  log "PocketID restaurado en ${POCKETID_DB_FILE}"
else
  warn "el backup no contiene pocket-id.db o se omitió con --no-pocketid: no se restauró la base de PocketID"
fi

start_app
APP_STOPPED=0

cat <<EOF

Restore completado
------------------
  Archivo        : $(basename "${ARCHIVE}")
  PostgreSQL     : restaurado con pg_restore --clean --if-exists
  PocketID       : $( [ "${INCLUDE_POCKETID}" -eq 1 ] && echo "restaurado" || echo "omitido" )
  Siguiente paso : verificá GET /ready y hacé un smoke test de la UI
EOF
