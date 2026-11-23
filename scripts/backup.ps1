<#
.SYNOPSIS
    HorneroDB — backup completo (PostgreSQL + PocketID/SQLite) para operadores Windows.

.DESCRIPTION
    Genera un archivo comprimido y timestamped con:
      * dump de PostgreSQL en formato custom (pg_dump --format=custom)
      * copia consistente de la base SQLite de PocketID (data/pocket-id.db)
      * manifiesto SHA256SUMS para verificar la integridad en el restore

    Es el equivalente de scripts/backup.sh: produce un .tar.gz compatible con
    scripts/restore.sh (usa tar.exe de Windows 10 1803+). Si tar no está
    disponible cae a Compress-Archive y genera un .zip.

.EXAMPLE
    .\scripts\backup.ps1
.EXAMPLE
    .\scripts\backup.ps1 -BackupDir D:\backups -RetentionDays 30
.EXAMPLE
    $env:DB_PASSWORD='secret'; .\scripts\backup.ps1 -BackupMode Direct
#>
[CmdletBinding()]
param(
    [string] $BackupDir      = $(if ($env:BACKUP_DIR) { $env:BACKUP_DIR } else { ".\backups" }),
    [int]    $RetentionDays  = $(if ($env:RETENTION_DAYS) { [int]$env:RETENTION_DAYS } else { 14 }),
    [ValidateSet('Auto', 'Docker', 'Direct')]
    [string] $BackupMode     = $(if ($env:BACKUP_MODE) { $env:BACKUP_MODE } else { 'Auto' }),
    [string] $ComposeFile    = $(if ($env:COMPOSE_FILE) { $env:COMPOSE_FILE } else { "docker-compose.yml" }),
    [string] $ComposeService = $(if ($env:COMPOSE_SERVICE) { $env:COMPOSE_SERVICE } else { "postgres" }),
    [string] $PocketIdDbFile = $(if ($env:POCKETID_DB_FILE) { $env:POCKETID_DB_FILE } else { "data\pocket-id.db" }),
    [switch] $NoPocketId
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ───────────────────────────── Configuración ─────────────────────────────

$DbHost     = $(if ($env:DB_HOST)     { $env:DB_HOST }     else { 'localhost' })
$DbPort     = $(if ($env:DB_PORT)     { $env:DB_PORT }     else { '5432' })
$DbUser     = $(if ($env:DB_USER)     { $env:DB_USER }     else { 'postgres' })
$DbPassword = $(if ($env:DB_PASSWORD) { $env:DB_PASSWORD } else { 'postgres' })
$DbName     = $(if ($env:DB_NAME)     { $env:DB_NAME }     else { 'hornero' })

$ProjectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $ProjectRoot
try {
    # ───────────────────────────── Utilidades ─────────────────────────────

    function Write-Log    { param([string]$Message) Write-Host "[backup] $Message" }
    function Write-Warn   { param([string]$Message) Write-Host "[backup] WARNING: $Message" -ForegroundColor Yellow }
    function Write-Fatal  { param([string]$Message) Write-Host "[backup] ERROR: $Message" -ForegroundColor Red; exit 1 }

    function Test-Command {
        param([string]$Name)
        return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
    }

    # Ejecuta un comando capturando la salida binaria en un fichero.
    # Redirección con '>' de PowerShell 5.1 corrompe los bytes: hay que usar
    # Start-Process con -RedirectStandardOutput para el dump de pg_dump.
    function Invoke-BinaryToFile {
        param(
            [Parameter(Mandatory)][string]   $FilePath,
            [Parameter(Mandatory)][string[]] $Arguments,
            [Parameter(Mandatory)][string]   $OutputFile
        )

        $errFile = "$OutputFile.stderr"
        $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments `
            -NoNewWindow -Wait -PassThru `
            -RedirectStandardOutput $OutputFile -RedirectStandardError $errFile

        if (Test-Path $errFile) {
            $stderr = (Get-Content $errFile -Raw -ErrorAction SilentlyContinue)
            if ($stderr) { Write-Verbose "stderr: $stderr" }
            Remove-Item $errFile -Force -ErrorAction SilentlyContinue
        }

        if ($process.ExitCode -ne 0) {
            Write-Fatal "el comando falló con código $($process.ExitCode): $FilePath $($Arguments -join ' ')"
        }
    }

    # ───────────────────────────── Detección de modo ─────────────────────────────

    if ($BackupMode -eq 'Auto') {
        if (Test-Command 'pg_dump') { $Mode = 'Direct' } else { $Mode = 'Docker' }
    } else {
        $Mode = $BackupMode
    }

    switch ($Mode) {
        'Docker' {
            if (-not (Test-Command 'docker')) { Write-Fatal "BackupMode=Docker pero docker no está instalado." }
            $composeCmd = $null
            if (Test-Command 'docker') {
                docker compose version *> $null
                if ($LASTEXITCODE -eq 0) { $composeCmd = @('docker', 'compose') }
            }
            if (-not $composeCmd) {
                if (Test-Command 'docker-compose') { $composeCmd = @('docker-compose') }
                else { Write-Fatal "docker compose no está disponible; instalalo o usá -BackupMode Direct." }
            }
        }
        'Direct' {
            if (-not (Test-Command 'pg_dump')) { Write-Fatal "BackupMode=Direct pero pg_dump no está en el PATH." }
        }
    }

    # ───────────────────────────── Preparación ─────────────────────────────

    $timestamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
    $staging   = Join-Path ([System.IO.Path]::GetTempPath()) ("hornerodb-backup-" + [guid]::NewGuid().ToString('N').Substring(0, 8))

    try {
        New-Item -ItemType Directory -Path $staging -Force | Out-Null
        New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null
        $backupDirAbs = (Resolve-Path $BackupDir).Path

        Write-Log "modo: $Mode"
        Write-Log "directorio de destino: $backupDirAbs"

        # ───────────────────────────── PostgreSQL ─────────────────────────────

        $dumpFile = Join-Path $staging 'postgres.dump'
        Write-Log "volcando PostgreSQL (base '$DbName')…"

        $env:PGPASSWORD = $DbPassword

        if ($Mode -eq 'Docker') {
            $prefix = if ($composeCmd.Length -gt 1) { @($composeCmd[1..($composeCmd.Length - 1)]) } else { @() }
            $args = @('-f', $ComposeFile, 'exec', '-T', $ComposeService, 'pg_dump',
                      '--format=custom', '--no-owner', '--no-privileges',
                      "--dbname=$DbName", "--username=$DbUser")
            Invoke-BinaryToFile -FilePath $composeCmd[0] -Arguments (@($prefix) + @($args)) -OutputFile $dumpFile
        } else {
            $args = @('--format=custom', '--no-owner', '--no-privileges',
                      "--host=$DbHost", "--port=$DbPort", "--username=$DbUser", "--dbname=$DbName")
            Invoke-BinaryToFile -FilePath 'pg_dump' -Arguments $args -OutputFile $dumpFile
        }

        if (-not (Test-Path $dumpFile) -or (Get-Item $dumpFile).Length -eq 0) {
            Write-Fatal "el dump de PostgreSQL está vacío: revisá la conexión y los permisos."
        }

        # ───────────────────────────── PocketID (SQLite) ─────────────────────────────

        $pocketIdIncluded = $false

        if (-not $NoPocketId) {
            if (-not (Test-Path $PocketIdDbFile)) {
                Write-Warn "no se encontró '$PocketIdDbFile'; se omite la base de PocketID (¿PocketID está habilitado?)"
            } else {
                Write-Log "copiando la base de PocketID ($PocketIdDbFile)…"
                $pocketTarget = Join-Path $staging 'pocket-id.db'
                $backedUpOnline = $false

                if (Test-Command 'sqlite3') {
                    # .backup produce una copia consistente incluyendo el WAL activo.
                    & sqlite3 $PocketIdDbFile ".backup '$pocketTarget'"
                    if ($LASTEXITCODE -eq 0) {
                        $backedUpOnline = $true
                    } else {
                        Write-Warn "sqlite3 .backup falló; intentando backup online vía Python sqlite3…"
                    }
                }

                if (-not $backedUpOnline) {
                    $pyCmd = if (Test-Command 'python3') { 'python3' } elseif (Test-Command 'python') { 'python' } else { $null }
                    if ($pyCmd) {
                        & $pyCmd -c "import sqlite3, sys; src=sqlite3.connect(sys.argv[1]); dst=sqlite3.connect(sys.argv[2]); src.backup(dst); dst.close(); src.close()" $PocketIdDbFile $pocketTarget
                        if ($LASTEXITCODE -eq 0) {
                            $backedUpOnline = $true
                        }
                    }
                }

                if (-not $backedUpOnline) {
                    Write-Warn "sqlite3/python no disponibles; copiando archivo principal junto a -wal/-shm si existen."
                    Copy-Item $PocketIdDbFile $pocketTarget -Force
                    foreach ($suffix in @('-wal', '-shm')) {
                        $sidecar = "$PocketIdDbFile$suffix"
                        if (Test-Path $sidecar) {
                            Copy-Item $sidecar (Join-Path $staging "pocket-id.db$suffix") -Force
                        }
                    }
                }

                $pocketIdIncluded = $true
            }
        }

        # ───────────────────────────── Checksums ─────────────────────────────

        Write-Log "calculando los checksums SHA-256…"

        $checksumLines = @()
        $candidateFiles = @('postgres.dump', 'pocket-id.db', 'pocket-id.db-wal', 'pocket-id.db-shm')
        $presentPayloadFiles = @()
        foreach ($name in $candidateFiles) {
            $path = Join-Path $staging $name
            if (Test-Path $path) {
                $hash = (Get-FileHash -Path $path -Algorithm SHA256).Hash.ToLowerInvariant()
                # Formato compatible con `sha256sum -c` (dos espacios).
                $checksumLines += "$hash  $name"
                $presentPayloadFiles += $name
            }
        }
        $checksumFile = Join-Path $staging 'SHA256SUMS'
        Set-Content -Path $checksumFile -Value $checksumLines -Encoding ascii

        # ───────────────────────────── Compresión ─────────────────────────────

        Write-Log "comprimiendo…"

        $items = @($presentPayloadFiles) + @('SHA256SUMS')

        if (Test-Command 'tar') {
            $archiveName = "hornerodb-backup-$timestamp.tar.gz"
            $archivePath = Join-Path $backupDirAbs $archiveName
            $tarArgs = @('-czf', $archivePath, '-C', $staging) + $items
            & tar @tarArgs
            if ($LASTEXITCODE -ne 0) { Write-Fatal "tar falló con código $LASTEXITCODE." }
        } else {
            Write-Warn "tar no está disponible (Windows 10 1803+ lo incluye); se genera un .zip. Para restaurarlo con scripts/restore.sh necesitás tar."
            $archiveName = "hornerodb-backup-$timestamp.zip"
            $archivePath = Join-Path $backupDirAbs $archiveName
            Compress-Archive -Path ($items | ForEach-Object { Join-Path $staging $_ }) -DestinationPath $archivePath -Force
        }

        if (-not (Test-Path $archivePath) -or (Get-Item $archivePath).Length -eq 0) {
            Write-Fatal "no se pudo crear el archivo de backup."
        }

        $sizeMb = [math]::Round((Get-Item $archivePath).Length / 1MB, 2)
        Write-Log "backup creado: $archivePath ($sizeMb MB)"

        # ───────────────────────────── Retención ─────────────────────────────

        Write-Log "aplicando retención ($RetentionDays días)…"

        $deleted = 0
        if ($RetentionDays -gt 0) {
            $cutoff = (Get-Date).ToUniversalTime().AddDays(-$RetentionDays)
            Get-ChildItem -Path $backupDirAbs -Filter 'hornerodb-backup-*.tar.gz' -File -ErrorAction SilentlyContinue |
                Where-Object { $_.LastWriteTimeUtc -lt $cutoff } |
                ForEach-Object {
                    Write-Log "  eliminado (obsoleto): $($_.Name)"
                    Remove-Item $_.FullName -Force
                    $deleted++
                }
            Get-ChildItem -Path $backupDirAbs -Filter 'hornerodb-backup-*.zip' -File -ErrorAction SilentlyContinue |
                Where-Object { $_.LastWriteTimeUtc -lt $cutoff } |
                ForEach-Object {
                    Write-Log "  eliminado (obsoleto): $($_.Name)"
                    Remove-Item $_.FullName -Force
                    $deleted++
                }
        }

        Write-Log "backups obsoletos eliminados: $deleted"

        Write-Host ""
        Write-Host "Resumen"
        Write-Host "-------"
        Write-Host "  Archivo      : $archivePath"
        Write-Host "  PostgreSQL   : dump en formato custom (pg_dump --format=custom)"
        Write-Host "  PocketID     : $(if ($pocketIdIncluded) { 'copiado' } else { 'omitido' })"
        Write-Host "  Checksums    : SHA256SUMS (verificados por scripts/restore.sh)"
        Write-Host "  Retención    : $RetentionDays días"
        Write-Host ""
        Write-Host "Restaurar con:"
        Write-Host "  ./scripts/restore.sh $archivePath"
    }
    finally {
        Remove-Item $staging -Recurse -Force -ErrorAction SilentlyContinue
    }
}
finally {
    Pop-Location
}
