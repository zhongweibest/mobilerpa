# adb-debug

该目录用于保存本地机器上的真机调试辅助脚本。

当前已提供：

1. 将 Agent 推送到调试手机。
2. 在需要时写入真机侧启动引导配置。
3. 尝试通过 AutoJs6 拉起 Agent 主入口。
4. 提供 `push-agent.bat` 交互式脚本，支持选择单台 ADB 设备、使用 `all` 批量推送，并记住上一次选择。

## 真机运行 Agent

### 1. 启动中心服务

真机访问电脑服务时，中心服务不能只监听 `127.0.0.1`。建议在中心服务仓库中这样启动：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-center\server
$env:CENTER_HTTP_ADDR="0.0.0.0:18080"
$env:CENTER_DB_PATH="D:\dev\code\mobilerpa\.tmp\mobilerpa-center-manual.db"
go run .\cmd\center
```

如果 Windows 防火墙询问是否允许访问，请允许当前网络访问。

### 2. 获取电脑局域网 IP

在电脑上执行：

```powershell
Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object { $_.IPAddress -notlike "127.*" -and $_.PrefixOrigin -ne "WellKnown" } |
  Select-Object IPAddress, InterfaceAlias
```

假设电脑 IP 是 `192.168.1.23`，那么手机访问中心服务的地址是：

```text
http://192.168.1.23:18080
```

注意：手机上不能使用 `http://127.0.0.1:18080`，因为手机里的 `127.0.0.1` 指向手机自己，不是电脑。

### 3. 确认 ADB 设备

```powershell
adb devices
```

如果有多台设备，`push-agent.bat` 会列出设备并让你选择；也可以输入 `all`，对当前所有已连接且已授权的真机批量推送。

### 4. 推送并运行 Agent

推荐使用 BAT 脚本执行，它会参考历史选择自动给出默认设备和中心地址：

```bat
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent
.\tools\adb-debug\push-agent.bat http://192.168.1.23:18080
```

脚本执行过程中按提示选择设备。如果需要重置真机配置，输入 `y`。

如果你希望对当前所有 ADB 已连接设备批量推送，可以在设备选择阶段输入：

```text
all
```

脚本会把上一次选择的设备、中心地址记录到 `%TEMP%\MobileRPA\push-agent.local.ini`，用于下次运行时提供默认值。

可选参数：

1. 第二个或第三个参数传 `reset`，表示覆盖真机配置并重新写入中心地址。
2. 第二个或第三个参数传 `norun`，表示只推送文件，不自动拉起 AutoJs6。

也可以在 Agent 仓库执行 PowerShell 脚本：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent
powershell -ExecutionPolicy Bypass -File .\tools\adb-debug\push-agent.ps1 `
  -CenterBaseURL "http://192.168.1.23:18080" `
  -Run
```

如果使用 PowerShell 脚本且有多台设备：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\adb-debug\push-agent.ps1 `
  -Serial "你的设备序列号" `
  -CenterBaseURL "http://192.168.1.23:18080" `
  -Run
```

脚本会把 Agent 推送到：

```text
/sdcard/脚本/agent/
```

首次启动引导文件会写入：

```text
/sdcard/脚本/agent/runtime/bootstrap.json
```

真实运行配置文件为：

```text
/sdcard/脚本/agent/runtime/config.json
```

说明：

1. 推送脚本默认不再直接推送 `config.json`。
2. 推送脚本每次都会刷新 `bootstrap.json`，用于同步最新的 `center_base_url` 和连接参数。
3. `agent.js` 启动时会读取 `bootstrap.json`，把新的中心地址合并到当前运行配置。
4. 如果真机已经存在 `config.json`，脚本仍会保留其中的 `agent_uuid` 和 `device_id`，不会因重推而丢失设备身份。
5. 当前 `all` 仅表示“本机当前 ADB 已连接且已授权的全部真机”，不等同于“中心服务中所有在线设备”。
6. 推送内容现在包含 `agent.js`、`lib/` 和 `scripts/`；新增脚本入口文件后，重新推送一次 Agent 即可同步到手机。

### 5. 重置真机配置

如果你需要重新设置中心地址，或者清空 `agent_uuid` 和 `device_id`，追加 `-ResetConfig`：

```bat
.\tools\adb-debug\push-agent.bat http://192.168.1.23:18080 reset
```

如果只想推送但不运行：

```bat
.\tools\adb-debug\push-agent.bat http://192.168.1.23:18080 norun
```

PowerShell 脚本重置方式：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\adb-debug\push-agent.ps1 `
  -CenterBaseURL "http://192.168.1.23:18080" `
  -ResetConfig `
  -Run
```

### 6. 手工运行方式

如果 `-Run` 没有成功拉起 AutoJs6，可以在 AutoJs6 中手工打开：

```text
/sdcard/脚本/agent/agent.js
```

如果你的 AutoJs6 包名或启动 Activity 不同，可以通过 `-AutoJsComponent` 覆盖，例如：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\adb-debug\push-agent.ps1 `
  -CenterBaseURL "http://192.168.1.23:18080" `
  -AutoJsComponent "org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity" `
  -Run
```

### 7. 验收结果确认

运行后在中心服务侧调用：

```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:18080/api/v1/devices" | ConvertTo-Json -Depth 8
```

预期结果：

1. 能看到真机注册出来的设备。
2. `agent_uuid` 不为空。
3. `device_id` 不为空。
4. 重复运行 Agent 后，`agent_uuid` 和 `device_id` 不应变化。
