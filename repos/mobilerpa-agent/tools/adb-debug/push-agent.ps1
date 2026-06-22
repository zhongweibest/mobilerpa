param(
    [string]$Adb = "adb",
    [string]$Serial = "",
    [Parameter(Mandatory = $true)]
    [string]$CenterBaseURL,
    [string]$RemoteRoot = "/sdcard/脚本",
    [string]$AutoJsComponent = "org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity",
    [switch]$ResetConfig,
    [switch]$Run
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..\..")
$AgentDir = Join-Path $RepoRoot "agent"
$AgentEntry = Join-Path $AgentDir "agent.js"
$AgentLibDir = Join-Path $AgentDir "lib"
$AgentScriptsDir = Join-Path $AgentDir "scripts"

if (-not (Test-Path $AgentEntry)) {
    throw "未找到 Agent 主入口：$AgentEntry"
}

if (-not (Test-Path $AgentLibDir)) {
    throw "未找到 Agent 依赖目录：$AgentLibDir"
}

if (-not (Test-Path $AgentScriptsDir)) {
    throw "未找到 Agent 脚本目录：$AgentScriptsDir"
}

$CommonAdbArgs = @()
if ($Serial.Trim() -ne "") {
    $CommonAdbArgs += @("-s", $Serial)
}

function Invoke-AgentAdb {
    param(
        [string[]]$Arguments
    )

    $AllArguments = @()
    $AllArguments += $CommonAdbArgs
    $AllArguments += $Arguments
    & $Adb @AllArguments
    if ($LASTEXITCODE -ne 0) {
        throw "ADB 命令执行失败：$Adb $($Arguments -join ' ')"
    }
}

$RemoteAgentDir = "$RemoteRoot/agent"
$RemoteRuntimeDir = "$RemoteAgentDir/runtime"
$RemoteConfigPath = "$RemoteRuntimeDir/config.json"
$RemoteBootstrapPath = "$RemoteRuntimeDir/bootstrap.json"
$RemoteEntryPath = "$RemoteAgentDir/agent.js"
$RemoteEntryUri = "file://" + $RemoteEntryPath

Write-Host "Agent: 准备推送到真机：$RemoteAgentDir"
Invoke-AgentAdb -Arguments @("shell", "mkdir", "-p", $RemoteAgentDir, $RemoteRuntimeDir)
Invoke-AgentAdb -Arguments @("push", $AgentEntry, "$RemoteAgentDir/agent.js")
Invoke-AgentAdb -Arguments @("push", $AgentLibDir, "$RemoteAgentDir/")
Invoke-AgentAdb -Arguments @("push", $AgentScriptsDir, "$RemoteAgentDir/")

if ($ResetConfig) {
    Invoke-AgentAdb -Arguments @("shell", "rm", "-f", $RemoteConfigPath)
}

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("mobilerpa-agent-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
$LocalBootstrapPath = Join-Path $TempDir "bootstrap.json"

$Bootstrap = [ordered]@{
    center_base_url = $CenterBaseURL
    websocket       = [ordered]@{
        enabled               = $true
        heartbeat_interval_ms = 30000
    }
}

$Bootstrap | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path $LocalBootstrapPath
Invoke-AgentAdb -Arguments @("push", $LocalBootstrapPath, $RemoteBootstrapPath)
Remove-Item -LiteralPath $TempDir -Recurse -Force

Write-Host "Agent: 已写入真机启动引导配置：$RemoteBootstrapPath"
Write-Host "Agent: 已同步 agent.js、lib 和 scripts 目录。"

if ($ResetConfig) {
    Write-Host "Agent: 已重置真机正式配置：$RemoteConfigPath"
} else {
    Write-Host "Agent: 已保留真机现有的 agent_uuid / device_id：$RemoteConfigPath"
}

Write-Host "Agent: 推送完成，Agent 入口：$RemoteEntryUri"

if ($Run) {
    Write-Host "Agent: 尝试通过 AutoJs6 运行 Agent。"
    Invoke-AgentAdb -Arguments @(
        "shell",
        "am",
        "start",
        "-n",
        $AutoJsComponent,
        "-d",
        $RemoteEntryUri
    )
} else {
    Write-Host "Agent: 已跳过自动运行，请在 AutoJs6 中手动打开：$RemoteEntryPath"
}
