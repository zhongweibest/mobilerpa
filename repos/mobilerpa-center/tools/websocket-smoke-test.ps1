param(
    [string]$ListenAddr = "127.0.0.1:18082",
    [string]$DBPath = "D:\dev\code\mobilerpa\.tmp\mobilerpa-center-ws-smoke.db",
    [string]$AgentUUID = "agent-ws-test-001"
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

function Send-WebSocketJson {
    param(
        [Parameter(Mandatory = $true)]
        [System.Net.WebSockets.ClientWebSocket]$Client,
        [Parameter(Mandatory = $true)]
        [object]$Body
    )

    $json = $Body | ConvertTo-Json -Depth 10 -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
    $segment = [System.ArraySegment[byte]]::new($bytes)
    [void]$Client.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
}

function Receive-WebSocketJson {
    param(
        [Parameter(Mandatory = $true)]
        [System.Net.WebSockets.ClientWebSocket]$Client
    )

    $buffer = New-Object byte[] 8192
    $segment = [System.ArraySegment[byte]]::new($buffer)
    $result = $Client.ReceiveAsync($segment, [Threading.CancellationToken]::None).GetAwaiter().GetResult()
    if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
        throw "websocket closed before ack"
    }

    $json = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $result.Count)
    return $json | ConvertFrom-Json
}

function Assert-AckOk {
    param(
        [Parameter(Mandatory = $true)]
        [object]$Ack,
        [Parameter(Mandatory = $true)]
        [string]$MessageType,
        [Parameter(Mandatory = $true)]
        [string]$RequestID
    )

    if ($Ack.type -ne "ack") {
        throw "expected ack type, got $($Ack.type)"
    }
    if ($Ack.request_id -ne $RequestID) {
        throw "expected request_id $RequestID, got $($Ack.request_id)"
    }
    if ($Ack.payload.message_type -ne $MessageType) {
        throw "expected message_type $MessageType, got $($Ack.payload.message_type)"
    }
    if ($Ack.payload.status -ne "ok") {
        throw "expected ack status ok, got $($Ack.payload.status)"
    }
}

function Get-DeviceByID {
    param(
        [Parameter(Mandatory = $true)]
        [object]$DevicesResponse,
        [Parameter(Mandatory = $true)]
        [string]$DeviceID
    )

    foreach ($device in $DevicesResponse.data) {
        if ($device.device_id -eq $DeviceID) {
            return $device
        }
    }

    throw "device $DeviceID not found in list response"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$serverDir = Join-Path $repoRoot "server"
$baseUrl = "http://$ListenAddr"
$wsUrl = "ws://$ListenAddr/ws"

if (-not (Test-Path (Split-Path -Parent $DBPath))) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $DBPath) | Out-Null
}

if (Test-Path $DBPath) {
    Remove-Item -LiteralPath $DBPath -Force
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

$client = $null

try {
    $health = Wait-ForHealthz -BaseUrl $baseUrl -Process $process

    $register = Invoke-JsonPost -Uri "$baseUrl/api/v1/device/register" -Body @{
        agent_uuid  = $AgentUUID
        device_name = "WebSocket Test"
        brand       = "Google"
        model       = "Pixel WS"
        android_id  = "android-ws-test-001"
    }
    $deviceID = $register.data.device_id

    $client = [System.Net.WebSockets.ClientWebSocket]::new()
    [void]$client.ConnectAsync([Uri]$wsUrl, [Threading.CancellationToken]::None).GetAwaiter().GetResult()

    $helloRequestID = "ws-hello-001"
    Send-WebSocketJson -Client $client -Body @{
        type       = "hello"
        request_id = $helloRequestID
        device_id  = $deviceID
        timestamp  = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        payload    = @{
            agent_uuid = $AgentUUID
        }
    }
    $helloAck = Receive-WebSocketJson -Client $client
    Assert-AckOk -Ack $helloAck -MessageType "hello" -RequestID $helloRequestID

    $heartbeatRequestID = "ws-heartbeat-001"
    Send-WebSocketJson -Client $client -Body @{
        type       = "heartbeat"
        request_id = $heartbeatRequestID
        device_id  = $deviceID
        timestamp  = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        payload    = @{
            battery = 90
        }
    }
    $heartbeatAck = Receive-WebSocketJson -Client $client
    Assert-AckOk -Ack $heartbeatAck -MessageType "heartbeat" -RequestID $heartbeatRequestID

    $onlineDevices = Invoke-JsonGet -Uri "$baseUrl/api/v1/devices"
    $onlineDevice = Get-DeviceByID -DevicesResponse $onlineDevices -DeviceID $deviceID
    if ($onlineDevice.status -ne "online") {
        throw "expected device status online, got $($onlineDevice.status)"
    }
    if ([string]::IsNullOrWhiteSpace($onlineDevice.last_heartbeat_at)) {
        throw "expected last_heartbeat_at to be set"
    }

    [void]$client.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "smoke done", [Threading.CancellationToken]::None).GetAwaiter().GetResult()
    $client.Dispose()
    $client = $null
    Start-Sleep -Milliseconds 500

    $offlineDevices = Invoke-JsonGet -Uri "$baseUrl/api/v1/devices"
    $offlineDevice = Get-DeviceByID -DevicesResponse $offlineDevices -DeviceID $deviceID
    if ($offlineDevice.status -ne "offline") {
        throw "expected device status offline after websocket close, got $($offlineDevice.status)"
    }

    [PSCustomObject]@{
        health        = $health
        register      = $register
        hello_ack     = $helloAck
        heartbeat_ack = $heartbeatAck
        online_device = $onlineDevice
        offline_device = $offlineDevice
    } | ConvertTo-Json -Depth 10
} finally {
    if ($client) {
        $client.Dispose()
    }
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
    }
}
