$ErrorActionPreference = "Stop"

$output = "dist/crawler-client.exe"
$src = "./cmd"
$iconPath = "icon.ico"
$sysoPath = "cmd/rsrc.syso"

# Ensure output directory exists
$outputDir = Split-Path $output -Parent
if ($outputDir -and -not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

# Handle Icon
if (Test-Path $iconPath) {
    Write-Host "Icon found ($iconPath), preparing resources..." -ForegroundColor Cyan
    
    if (-not (Get-Command "rsrc" -ErrorAction SilentlyContinue)) {
        Write-Host "Installing rsrc tool..." -ForegroundColor Yellow
        go install github.com/akavel/rsrc@latest
    }

    # Generate .syso file
    # -arch amd64 is important for 64-bit builds
    Write-Host "Generating resource file..."
    rsrc -ico $iconPath -arch amd64 -o $sysoPath
} else {
    Write-Host "No icon.ico found, skipping icon embedding." -ForegroundColor Gray
}

Write-Host "Compiling for Windows (amd64)..." -ForegroundColor Cyan

# Set environment variables
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Build command
# -ldflags="-s -w": Strip debug information to reduce size
# -trimpath: Remove file system paths from executable
try {
    go build -ldflags="-s -w" -trimpath -o $output $src
} finally {
    # Cleanup .syso file to keep source clean
    if (Test-Path $sysoPath) {
        Remove-Item $sysoPath
    }
}

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build success!" -ForegroundColor Green

    # Copy necessary files to dist
    Write-Host "Copying config and key files..." -ForegroundColor Cyan
    
    if (Test-Path "public.pem") {
        Copy-Item "public.pem" -Destination $outputDir
        Write-Host "  [+] public.pem copied" -ForegroundColor Green
    } else {
        Write-Host "  [!] public.pem not found! Client may fail to validate tokens." -ForegroundColor Red
    }

    if (Test-Path "user_config.json") {
        Copy-Item "user_config.json" -Destination $outputDir
        Write-Host "  [+] user_config.json copied" -ForegroundColor Green
    } else {
        # Create a default config if not exists
        $defaultConfig = @{
            server_url = "http://localhost:8080"
            token = ""
            proxy_host = "127.0.0.1"
            proxy_port = 7890
            base_dir = ""
        } | ConvertTo-Json
        $defaultConfig | Out-File -FilePath "$outputDir/user_config.json" -Encoding utf8
        Write-Host "  [+] Created default user_config.json" -ForegroundColor Green
    }

    $item = Get-Item $output
    $sizeMB = $item.Length / 1MB
    Write-Host ("Output file: {0}" -f $item.FullName)
    Write-Host ("File size: {0:N2} MB" -f $sizeMB)
    
    # Check for UPX
    if (Get-Command "upx" -ErrorAction SilentlyContinue) {
        Write-Host "UPX detected, compressing..." -ForegroundColor Cyan
        upx --best $output
        $newItem = Get-Item $output
        $newSizeMB = $newItem.Length / 1MB
        Write-Host ("Compressed size: {0:N2} MB (Reduced by {1:P0})" -f $newSizeMB, (1 - $newSizeMB/$sizeMB)) -ForegroundColor Green
    } else {
        Write-Host "Tip: Install UPX (https://github.com/upx/upx) to further reduce binary size." -ForegroundColor Yellow
    }
} else {
    Write-Host "Build failed." -ForegroundColor Red
    exit 1
}
