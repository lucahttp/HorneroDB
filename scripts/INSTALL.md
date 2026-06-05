# Instalar el bloque HorneroDB en Typebot

Guía paso a paso (sin scripts).

## Requisitos

- `git` instalado
- `pnpm` instalado (`npm install -g pnpm`)
- `node >= 20`
- `docker` (para levantar el stack final)

---

## Paso 1: Clonar typebot.io

```bash
cd ~/workspace  # o tu carpeta de trabajo

git clone https://github.com/baptisteArno/typebot.io.git typebot-fork
cd typebot-fork

git checkout -b hornerodb-block
```

---

## Paso 2: Clonar o ubicar HorneroDB

Si no lo tenés clonado:

```bash
cd ~/workspace
git clone https://github.com/lucahttp/HorneroDB.git
```

Si ya lo tenés:
```bash
# Simplemente anótate dónde está
# Ej: /c/Users/lucas/HorneroDB
```

---

## Paso 3: Copiar el bloque a typebot

Desde dentro de `typebot-fork`:

```bash
mkdir -p packages/forge/blocks/hornerodb

# Reemplazar HORNERO_PATH con la ruta real de tu HorneroDB
cp -r HORNERO_PATH/docs/connectors/typebot/* \
      packages/forge/blocks/hornerodb/
```

**Ejemplo real:**
```bash
cp -r /c/Users/lucas/HorneroDB/docs/connectors/typebot/* \
      packages/forge/blocks/hornerodb/
```

---

## Paso 4: Arreglar tsconfig.json

Abrí `packages/forge/blocks/hornerodb/tsconfig.json` y reemplazá TODO el contenido por:

```json
{
  "extends": "../../tsconfig.base.json",
  "files": [],
  "include": [],
  "references": [
    { "path": "./tsconfig.lib.json" }
  ]
}
```

---

## Paso 5: Arreglar tsconfig.lib.json

Abrí `packages/forge/blocks/hornerodb/tsconfig.lib.json` y reemplazá TODO por:

```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "rootDir": "src",
    "outDir": "dist",
    "tsBuildInfoFile": "dist/tsconfig.lib.tsbuildinfo"
  },
  "include": ["src/**/*.ts", "src/**/*.tsx"],
  "references": [
    { "path": "../../lib/tsconfig.lib.json" },
    { "path": "../core/tsconfig.lib.json" }
  ]
}
```

---

## Paso 6: Actualizar package.json

Abrí `packages/forge/blocks/hornerodb/package.json`.

Encontrá la línea que dice:
```json
"name": "@typebot.io/hornerodb-block",
```

Si no dice exactamente eso, cámbialo. (Probablemente diga algo como `"name": "@typebot.io/hornerodb-block"` ya, pero verificá.)

---

## Paso 7: Instalar dependencias

Desde la raíz de `typebot-fork`:

```bash
pnpm install
```

Esto tarda 3-5 minutos la primera vez. Dejalo que termine.

---

## Paso 8: Compilar el bloque

```bash
pnpm --filter "@typebot.io/hornerodb-block" build
```

---

## Paso 9: Actualizar tu docker-compose

Abrí el `docker-compose.yml` de tu stack de typebot (donde está el servidor corriendo).

Ubicá los servicios `typebot-builder` y `typebot-viewer`.

**Antes:**
```yaml
typebot-builder:
  build:
    context: .
    dockerfile: apps/builder/Dockerfile
  ...
```

**Después:**
```yaml
typebot-builder:
  build:
    context: ../typebot-fork
    dockerfile: apps/builder/Dockerfile
  ...
```

Hacé lo mismo para `typebot-viewer`.

---

## Paso 10: Reconstruir imágenes

Desde tu carpeta de `typebot-stack`:

```bash
docker compose build typebot-builder typebot-viewer
```

---

## Paso 11: Levantar

```bash
docker compose up -d
```

---

## Verificar que funciona

1. Abrí `https://typebot.tu-dominio.com`
2. Abrí o creaá un bot
3. Click en "Agregar bloque"
4. En el sidebar izquierdo → busca la sección **Forge**
5. Debería aparecer **"HorneroDB"** con el logo naranja

Si no aparece → Paso 12 (troubleshooting).

---

## Paso 12: Configurar el bloque

Una vez que ves la card HorneroDB:

1. Click en **"HorneroDB"**
2. Te pide **Base URL**:
   - Poné: `https://hornero.tu-dominio.com` (o la URL real de tu HorneroDB)
3. Te pide **API Key**:
   - Generá una en HorneroDB → Workspace → API Keys
   - Poné la key generada

---

## Troubleshooting

### El bloque no aparece en el editor

```bash
cd ~/workspace/typebot-fork
pnpm nx reset
pnpm install
```

Después:
```bash
# Volvé a tu docker-stack
docker compose down
docker compose build typebot-builder typebot-viewer
docker compose up -d
```

### Las actions devuelven error 401

La API key no tiene permisos. En HorneroDB:

1. Workspace → API Keys
2. Click en la key que usaste
3. Verificá que el **Rol** tenga permisos sobre la tabla que estás usando
4. Si no, creá una key nueva con permisos

### Las actions devuelven error 403 CORS

En el `.env` de HorneroDB, agregá esta línea:

```
CORS_ORIGINS=https://typebot.tu-dominio.com,https://bot.tu-dominio.com
```

Después reiniciá HorneroDB:
```bash
docker compose restart hornerodb-server
```

### pnpm install falla

```bash
pnpm store prune
pnpm install
```

---

## Actualizar el bloque

Si hacés cambios en `HorneroDB/docs/connectors/typebot/`:

1. Copia nuevamente desde Step 3
2. Recompila desde Step 8
3. Reconstruye docker desde Step 10
4. Levantá desde Step 11

---

## Estructura final

```
~/workspace/
├── HorneroDB/                     # tu repo
│   └── docs/connectors/typebot/   # fuente del bloque
├── typebot-fork/                  # clonado en Paso 1
│   └── packages/forge/blocks/hornerodb/  # copia del bloque
└── typebot-stack/                 # tu stack de producción
    ├── docker-compose.yml         # editado en Paso 9
    └── .env
```
