param(
    [string]$ListenAddr = "127.0.0.1:18080",
    [string]$DBPath = "D:\dev\code\mobilerpa\.tmp\mobilerpa-center-smoke.db",
    [string]$AgentUUID = "agent-test-001"
)

$ErrorActionPreference = "Stop"

function Invoke-JsonGet {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri
    )

    return Invoke-RestMethod -Uri $Uri -Method Get
}

function Invoke-JsonPost {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,
        [Parameter(Mandatory = $true)]
        [object]$Body
    )

    $json = $Body | ConvertTo-Json -Depth 10
    return Invoke-RestMethod -Uri $Uri -Method Post -ContentType "application/json" -Body $json
}

function Wait-ForHealthz {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BaseUrl,
        [Parameter(Mandatory = $true)]
        [System.Diagnostics.Process]$Process,
        [int]$MaxAttempts = 20
    )

    for ($i = 1; $i -le $MaxAttempts; $i++) {
        if ($Process.HasExited) {
            $stdout = $Process.StandardOutput.ReadToEnd()
            $stderr = $Process.StandardError.ReadToEnd()
            throw "center service exited early.`nSTDOUT:`n$stdout`nSTDERR:`n$stderr"
        }

        try {
            return Invoke-JsonGet -Uri "$BaseUrl/healthz"
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }

    $stdout = $Process.StandardOutput.ReadToEnd()
    $stderr = $Process.StandardError.ReadToEnd()
    throw "center service did not become healthy in time.`nSTDOUT:`n$stdout`nSTDERR:`n$stderr"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$serverDir = Join-Path $repoRoot "server"
$baseUrl = "http://$ListenAddr"

if (-not (Test-Path (Split-Path -Parent $DBPath))) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $DBPath) | Out-Null
}

$command = @"
`$env:CENTER_HTTP_ADDR='$ListenAddr'
`$env:CENTER_DB_PATH='$DBPath'
`$env:GOCACHE='D:\dev\code\mobilerpa\.tmp\gocache'
`$env:GOPROXY='https://goproxy.cn,direct'
Set-Location '$serverDir'
go run .\cmd\center
"@

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "powershell.exe"
$psi.Arguments = "-NoProfile -Command $command"
$psi.WorkingDirectory = $serverDir
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true

$process = New-Object System.Diagnostics.Process
$process.StartInfo = $psi
[void]$process.Start()

try {
    $health = Wait-ForHealthz -BaseUrl $baseUrl -Process $process

    $register = Invoke-JsonPost -Uri "$baseUrl/api/v1/device/register" -Body @{
        agent_uuid  = $AgentUUID
        device_name = "Pixel Test"
        brand       = "Google"
        model       = "Pixel 8"
        android_id  = "android-test-001"
    }

    $devices = Invoke-JsonGet -Uri "$baseUrl/api/v1/devices"
    $device = Invoke-JsonGet -Uri "$baseUrl/api/v1/devices/$($register.data.device_id)"

    [PSCustomObject]@{
        health   = $health
        register = $register
        devices  = $devices
        device   = $device
    } | ConvertTo-Json -Depth 10
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
    }
}
