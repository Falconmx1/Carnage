# build.ps1 - Compila Carnage para múltiples plataformas (Windows PowerShell).
param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$OutDir = "dist"
$LdFlags = "-s -w -X main.Version=$Version"

$Platforms = @(
    @{ GOOS = "linux";   GOARCH = "amd64"; GOARM = "" },
    @{ GOOS = "linux";   GOARCH = "arm64"; GOARM = "" },
    @{ GOOS = "linux";   GOARCH = "arm";   GOARM = "7" },
    @{ GOOS = "linux";   GOARCH = "mips";  GOARM = "" },
    @{ GOOS = "linux";   GOARCH = "mipsle";GOARM = "" },
    @{ GOOS = "windows"; GOARCH = "amd64"; GOARM = "" },
    @{ GOOS = "windows"; GOARCH = "arm64"; GOARM = "" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; GOARM = "" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; GOARM = "" }
)

if (Test-Path $OutDir) { Remove-Item -Recurse -Force $OutDir }
New-Item -ItemType Directory -Path $OutDir | Out-Null

Write-Host "[*] Compilando Carnage $Version..."
foreach ($p in $Platforms) {
    $name = "carnage-$($p.GOOS)-$($p.GOARCH)"
    if ($p.GOARM -ne "") { $name = "$name" + "v$($p.GOARM)" }
    if ($p.GOOS -eq "windows") { $name = "$name.exe" }

    Write-Host "    -> $name"
    $env:CGO_ENABLED = "0"
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    if ($p.GOARM -ne "") { $env:GOARM = $p.GOARM } else { Remove-Item Env:GOARM -ErrorAction SilentlyContinue }

    go build -trimpath -ldflags $LdFlags -o "$OutDir/$name" ./cmd/carnage
    if ($LASTEXITCODE -ne 0) { throw "Fallo compilando $name" }
}

Write-Host "[+] Listo. Binarios en $OutDir/"
Get-ChildItem $OutDir | Format-Table Name, Length
