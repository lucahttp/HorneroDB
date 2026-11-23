#!/usr/bin/env bash
#
# HorneroDB — backup completo (PostgreSQL + PocketID/SQLite).
#
# Genera un archivo comprimido y timestamped con:
#   * dump de PostgreSQL en formato custom (pg_dump --format=custom)
#   * copia consistente de la base SQLite de PocketID (data/pocket-id.db)
#   * manifiesto SHA256SUMS para verificar la integridad en el restore
#
# Uso:
#   ./scripts/backup.sh [--dir BACKUP_DIR] [--retention DAYS] [--no-pocketid]
#
# Variables de entorno relevantes:
#   DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
#   POCKETID_DB_FILE   Ruta de la base SQLite de PocketID (por defecto data/pocket-id.db)
#   BACKUP_DIR         Directorio de destino (por defecto ./backups)
#   RETENTION_DAYS     Días de retención de los backups (por defecto 14)
#   BACKUP_MODE        auto | docker | direct (por defecto auto)
#   COMPOSE_FILE       Fichero compose alternativo (por defecto docker-compose.yml)
#   COMPOSE_SERVICE    Servicio de PostgreSQL en compose (por defecto postgres)
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
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
BACKUP_MODE="${BACKUP_MODE:-auto}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"

INCLUDE_POCKETID=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ───────────────────────────── Utilidades ─────────────────────────────

log()  { printf '[backup] %s\n' "$*"; }
warn() { printf '[backup] WARNING: %s\n' "$*" >&2; }
die()  { printf '[backup] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
  sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  exit 0
}

# sha256sum no existe en todos los sistemas: cae a shasum (macOS) o openssl.
sha256_of() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file"
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | sed 's/^.*= *//'
  else
    die "no se encontró sha256sum, shasum ni openssl para calcular los checksums"
  fi
}

# ───────────────────────────── Argumentos ─────────────────────────────

while [ $# -gt 0 ]; do
  case "$1" in
    --dir)        BACKUP_DIR="$2"; shift 2 ;;
    --retention)  RETENTION_DAYS="$2"; shift 2 ;;
    --no-pocketid) INCLUDE_POCKETID=0; shift ;;
    -h|--help)    usage ;;
    *)            die "argumento desconocido: $1 (usá --help)" ;;
  esac
done

case "$RETENTION_DAYS" in
  ''|*[!0-9]*) die "RETENTION_DAYS debe ser un número entero (valor actual: '$RETENTION_DAYS')" ;;
esac

# ───────────────────────────── Detección de modo ─────────────────────────────

compose() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    docker compose -f "${COMPOSE_FILE}" "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose -f "${COMPOSE_FILE}" "$@"
  else
    die "docker compose no está disponible; instalalo o usá BACKUP_MODE=direct con pg_dump en el PATH"
  fi
}

detect_mode() {
  if [ "${BACKUP_MODE}" != "auto" ]; then
    echo "${BACKUP_MODE}"
    return
  fi
  if command -v pg_dump >/dev/null 2>&1; then
    echo "direct"
  else
    echo "docker"
  fi
}

MODE="$(detect_mode)"

case "${MODE}" in
  docker)
    command -v docker >/dev/null 2>&1 || die "BACKUP_MODE=docker pero docker no está instalado"
    ;;
  direct)
    command -v pg_dump >/dev/null 2>&1 || die "BACKUP_MODE=direct pero pg_dump no está en el PATH"
    ;;
  *)
    die "BACKUP_MODE inválido: '${MODE}' (valores: auto, docker, direct)"
    ;;
esac

# ───────────────────────────── Preparación ─────────────────────────────

cd "${PROJECT_ROOT}"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE_NAME="hornerodb-backup-${TIMESTAMP}.tar.gz"
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/hornerodb-backup.XXXXXX")"

cleanup() {
  rm -rf "${STAGING_DIR}"
}
trap cleanup EXIT

mkdir -p "${BACKUP_DIR}"
BACKUP_DIR_ABS="$(cd "${BACKUP_DIR}" && pwd)"

log "modo: ${MODE}"
log "directorio de destino: ${BACKUP_DIR_ABS}"
log "archivo: ${ARCHIVE_NAME}"

# ───────────────────────────── PostgreSQL ─────────────────────────────

PG_DUMP_FILE="${STAGING_DIR}/postgres.dump"

log "volcando PostgreSQL (base '${DB_NAME}')…"

if [ "${MODE}" = "docker" ]; then
  # -T desactiva el TTY: imprescindible para el formato binario de pg_dump.
  PGPASSWORD="${DB_PASSWORD}" compose exec -T "${COMPOSE_SERVICE}" \
    pg_dump \
      --format=custom \
      --no-owner \
      --no-privileges \
      --dbname="${DB_NAME}" \
      --username="${DB_USER}" \
    > "${PG_DUMP_FILE}"
else
  PGPASSWORD="${DB_PASSWORD}" pg_dump \
    --format=custom \
    --no-owner \
    --no-privileges \
    --host="${DB_HOST}" \
    --port="${DB_PORT}" \
    --username="${DB_USER}" \
    --dbname="${DB_NAME}" \
    > "${PG_DUMP_FILE}"
fi

[ -s "${PG_DUMP_FILE}" ] || die "el dump de PostgreSQL está vacío: revisá la conexión y los permisos"

# ───────────────────────────── PocketID (SQLite) ─────────────────────────────

POCKETID_FILE=""

if [ "${INCLUDE_POCKETID}" -eq 1 ]; then
  if [ ! -f "${POCKETID_DB_FILE}" ]; then
    warn "no se encontró '${POCKETID_DB_FILE}'; se omite la base de PocketID (¿PocketID está habilitado?)"
  else
    POCKETID_FILE="${STAGING_DIR}/pocket-id.db"
    log "copiando la base de PocketID (${POCKETID_DB_FILE})…"

    if command -v sqlite3 >/dev/null 2>&1; then
      # .backup produce una copia consistente incluyendo el WAL activo.
      sqlite3 "${POCKETID_DB_FILE}" ".backup '${POCKETID_FILE}'"
    elif command -v python3 >/dev/null 2>&1; then
      python3 -c 'import sqlite3, sys; src=sqlite3.connect(sys.argv[1]); dst=sqlite3.connect(sys.argv[2]); src.backup(dst); dst.close(); src.close()' "${POCKETID_DB_FILE}" "${POCKETID_FILE}"
    else
      warn "sqlite3 y python3 no están instalados; copiando archivo principal junto a -wal/-shm si existen"
      TMP_COPY="${POCKETID_DB_FILE}.backup.$$"
      cp -p "${POCKETID_DB_FILE}" "${TMP_COPY}"
      mv -f "${TMP_COPY}" "${POCKETID_FILE}"
      for suffix in -wal -shm; do
        if [ -f "${POCKETID_DB_FILE}${suffix}" ]; then
          cp -p "${POCKETID_DB_FILE}${suffix}" "${POCKETID_FILE}${suffix}"
        fi
      done
    fi

    [ -s "${POCKETID_FILE}" ] || warn "la copia de PocketID quedó vacía"
  fi
fi

# ───────────────────────────── Checksums ─────────────────────────────

log "calculando los checksums SHA-256…"

CHECKSUM_FILE="${STAGING_DIR}/SHA256SUMS"
: > "${CHECKSUM_FILE}"

for f in postgres.dump pocket-id.db pocket-id.db-wal pocket-id.db-shm; do
  if [ -f "${STAGING_DIR}/${f}" ]; then
    ( cd "${STAGING_DIR}" && sha256_of "${f}" ) >> "${CHECKSUM_FILE}"
  fi
done

# ───────────────────────────── Compresión ─────────────────────────────

log "comprimiendo…"

ARCHIVE_PATH="${BACKUP_DIR_ABS}/${ARCHIVE_NAME}"

tar -czf "${ARCHIVE_PATH}" \
  -C "${STAGING_DIR}" \
  postgres.dump \
  SHA256SUMS \
  $( [ -f "${POCKETID_FILE}" ] && echo "pocket-id.db" )

[ -s "${ARCHIVE_PATH}" ] || die "no se pudo crear el archivo de backup"

chmod 600 "${ARCHIVE_PATH}"

log "backup creado: ${ARCHIVE_PATH} ($(du -h "${ARCHIVE_PATH}" | cut -f1))"

# ───────────────────────────── Retención ─────────────────────────────

log "aplicando retención (${RETENTION_DAYS} días)…"

DELETED=0
if [ "${RETENTION_DAYS}" -gt 0 ]; then
  while IFS= read -r old; do
    [ -n "${old}" ] || continue
    rm -f "${old}"
    DELETED=$((DELETED + 1))
    log "  eliminado (obsoleto): $(basename "${old}")"
  done <<EOF
$(find "${BACKUP_DIR_ABS}" -maxdepth 1 -type f -name 'hornerodb-backup-*.tar.gz' -mtime "+${RETENTION_DAYS}" 2>/dev/null || true)
EOF
fi

log "backups obsoletos eliminados: ${DELETED}"
log "listo."

cat <<EOF

Resumen
-------
  Archivo      : ${ARCHIVE_PATH}
  PostgreSQL   : dump en formato custom (pg_dump --format=custom)
  PocketID     : $( [ -f "${POCKETID_FILE}" ] && echo "copiado" || echo "omitido" )
  Checksums    : SHA256SUMS (verificados por scripts/restore.sh)
  Retención    : ${RETENTION_DAYS} días

Restaurar con:
  ./scripts/restore.sh ${ARCHIVE_PATH}
EOF
