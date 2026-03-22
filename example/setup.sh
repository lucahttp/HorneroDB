#!/bin/bash

# Asegurarnos de estar en el directorio correcto
cd "$(dirname "$0")"

ENV_FILE=".env"

if [ ! -f "$ENV_FILE" ]; then
    echo "❌ Error: No se encontró el archivo $ENV_FILE en esta carpeta."
    exit 1
fi

echo "Generando nuevos secretos criptográficos..."

# Generar las nuevas claves usando openssl (requiere openssl instalado)
NEW_JWT=$(openssl rand -base64 32)
NEW_ENC=$(openssl rand -base64 32)
# Generamos una contraseña hexadecimal (más segura al no tener caracteres raros que rompan la base de datos)
NEW_PG=$(openssl rand -hex 24)

# Reemplazar en el archivo .env 
# (Usamos -i.bak para que funcione tanto en Linux como en macOS/Git Bash en Windows)
sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=${NEW_JWT}|g" "$ENV_FILE"
sed -i.bak "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=${NEW_ENC}|g" "$ENV_FILE"
sed -i.bak "s|^DB_PASSWORD=.*|DB_PASSWORD=${NEW_PG}|g" "$ENV_FILE"
sed -i.bak "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${NEW_PG}|g" "$ENV_FILE"

# Eliminar el archivo de backup automático
rm -f "${ENV_FILE}.bak"

echo "✅ Secretos actualizados exitosamente en $ENV_FILE."
echo "JWT_SECRET, ENCRYPTION_KEY y contraseñas de PostgreSQL ahora son aleatorias y únicas."
