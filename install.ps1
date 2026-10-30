# Installs claude-gauges: downloads the release for this system, checks its checksum, then runs
# `claude-gauges install`, which copies the program to ~/.claude and sets the Claude Code status line.
#
#   irm https://raw.githubusercontent.com/mountaser/claude-gauges/main/install.ps1 | iex
#
# Set $env:CLAUDE_GAUGES_VERSION (for example v0.2.1) to install a specific release instead of the latest.
# Set $env:CLAUDE_GAUGES_ARGS (for example --force) to pass flags to `claude-gauges install`.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'mountaser/claude-gauges'

# PROCESSOR_ARCHITEW6432 holds the real CPU when a 32-bit PowerShell runs on 64-bit Windows.
$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($cpu) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "claude-gauges install: unsupported CPU '$cpu'" }
}
$asset = "claude-gauges_windows_$arch.exe"

$version = if ($env:CLAUDE_GAUGES_VERSION) { $env:CLAUDE_GAUGES_VERSION } else { 'latest' }
$base = if ($version -eq 'latest') {
    "https://github.com/$repo/releases/latest/download"
} else {
    "https://github.com/$repo/releases/download/$version"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ('claude-gauges-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $exe = Join-Path $tmp 'claude-gauges.exe'
    $sums = Join-Path $tmp 'checksums.txt'

    Write-Host "Downloading $asset ($version)..."
    Invoke-WebRequest -UseBasicParsing "$base/$asset" -OutFile $exe
    Invoke-WebRequest -UseBasicParsing "$base/checksums.txt" -OutFile $sums

    $line = Get-Content $sums | Where-Object { $_ -match ('\s\*?' + [regex]::Escape($asset) + '$') } | Select-Object -First 1
    if (-not $line) { throw "claude-gauges install: no checksum is listed for $asset" }
    $want = ($line -split '\s+')[0].ToLower()
    $got = (Get-FileHash -Algorithm SHA256 $exe).Hash.ToLower()
    if ($want -ne $got) { throw "claude-gauges install: checksum mismatch for $asset (expected $want, got $got)" }

    $extra = if ($env:CLAUDE_GAUGES_ARGS) { $env:CLAUDE_GAUGES_ARGS -split '\s+' } else { @() }
    & $exe install @extra
    if ($LASTEXITCODE -ne 0) { throw "claude-gauges install failed (exit code $LASTEXITCODE)" }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
