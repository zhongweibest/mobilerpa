@echo off
chcp 65001 >nul
setlocal EnableExtensions EnableDelayedExpansion

set "CONFIG_DIR=%TEMP%\MobileRPA"
if not defined TEMP set "CONFIG_DIR=%~dp0"
if not exist "%CONFIG_DIR%" mkdir "%CONFIG_DIR%" >nul 2>nul
if not exist "%CONFIG_DIR%" set "CONFIG_DIR=%TEMP%"
set "CONFIG_FILE=%CONFIG_DIR%\push-agent.local.ini"
set "REMOTE_ROOT=/sdcard/脚本"
set "AUTOJS_COMPONENT=org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity"
set "LAST_DEVICE="
set "LAST_CENTER_BASE_URL="
set "LAST_AUTOJS_COMPONENT="
set "TARGET_ALL=N"

for %%I in ("%~dp0..\..") do set "REPO_ROOT=%%~fI"
set "AGENT_DIR=%REPO_ROOT%\agent"
set "AGENT_ENTRY=%AGENT_DIR%\agent.js"
set "AGENT_LIB_DIR=%AGENT_DIR%\lib"
set "AGENT_SCRIPTS_DIR=%AGENT_DIR%\scripts"

if exist "%CONFIG_FILE%" (
    for /f "usebackq tokens=1,* delims==" %%A in ("%CONFIG_FILE%") do (
        if /i "%%A"=="DEVICE" set "LAST_DEVICE=%%B"
        if /i "%%A"=="CENTER_BASE_URL" set "LAST_CENTER_BASE_URL=%%B"
        if /i "%%A"=="AUTOJS_COMPONENT" set "LAST_AUTOJS_COMPONENT=%%B"
    )
)

if defined LAST_AUTOJS_COMPONENT set "AUTOJS_COMPONENT=%LAST_AUTOJS_COMPONENT%"
if not "%~1"=="" set "LAST_CENTER_BASE_URL=%~1"

if not exist "%AGENT_ENTRY%" (
    echo 未找到 Agent 主入口：%AGENT_ENTRY%
    exit /b 1
)

if not exist "%AGENT_LIB_DIR%\" (
    echo 未找到 Agent 依赖目录：%AGENT_LIB_DIR%
    exit /b 1
)

if not exist "%AGENT_SCRIPTS_DIR%\" (
    echo 未找到 Agent 脚本目录：%AGENT_SCRIPTS_DIR%
    exit /b 1
)

set "deviceCount=0"
echo ===========================
echo Available devices:
echo ===========================
for /f "skip=1 tokens=1,2" %%A in ('adb devices') do (
    if /i "%%B"=="device" (
        set /a deviceCount+=1
        set "device[!deviceCount!]=%%A"
        echo !deviceCount!. %%A
    ) else if /i "%%B"=="unauthorized" (
        echo 跳过未授权设备：%%A [unauthorized]
    )
)

if "!deviceCount!"=="0" (
    echo 没有找到可用设备，请先连接手机并确认 USB 调试授权。
    exit /b 1
)

set "DEFAULT_DEVICE=!device[1]!"
for /l %%I in (1,1,!deviceCount!) do (
    if /i "!device[%%I]!"=="%LAST_DEVICE%" set "DEFAULT_DEVICE=%LAST_DEVICE%"
)

:ChooseDevice
echo.
set "choice="
set /p "choice=Choose device number or all, default %DEFAULT_DEVICE%: "
set "choiceRaw=%choice%"
for /f "tokens=* delims= " %%A in ("%choiceRaw%") do set "choice=%%A"
if not defined choice (
    set "DEVICE=%DEFAULT_DEVICE%"
    goto DeviceSelected
)

for /f "tokens=1" %%A in ("!choice!") do set "choice=%%A"
if /i "!choice!"=="all" (
    set "TARGET_ALL=Y"
    set "DEVICE=all"
    goto DeviceSelected
)

set "DEVICE="
for /l %%I in (1,1,!deviceCount!) do (
    if "!choice!"=="%%I" set "DEVICE=!device[%%I]!"
    if /i "!choice!"=="!device[%%I]!" set "DEVICE=!device[%%I]!"
)
if not defined DEVICE (
    echo 输入无效：!choice!
    goto ChooseDevice
)

:DeviceSelected
if /i "%TARGET_ALL%"=="Y" (
    echo Selected devices: all authorized devices
) else (
    echo Selected device: %DEVICE%
)

echo.
set "centerInput="
if defined LAST_CENTER_BASE_URL (
    set /p "centerInput=Center base URL, default %LAST_CENTER_BASE_URL%: "
) else (
    echo 示例：http://192.168.1.23:18080
    set /p "centerInput=Center base URL: "
)
set "centerInputRaw=%centerInput%"
for /f "tokens=* delims= " %%A in ("%centerInputRaw%") do set "centerInput=%%A"
if not defined centerInput (
    set "CENTER_BASE_URL=%LAST_CENTER_BASE_URL%"
) else (
    set "CENTER_BASE_URL=%centerInput%"
)

if not defined CENTER_BASE_URL (
    echo 未输入中心服务地址。
    exit /b 1
)

set "RESET_CONFIG=N"
if /i "%~2"=="reset" set "RESET_CONFIG=Y"
if /i "%~3"=="reset" set "RESET_CONFIG=Y"
if /i "%RESET_CONFIG%"=="N" (
    set "resetChoice="
    set /p "resetChoice=是否重置真机配置？[y/N]: "
    if /i "!resetChoice!"=="y" set "RESET_CONFIG=Y"
    if /i "!resetChoice!"=="yes" set "RESET_CONFIG=Y"
)

set "RUN_AGENT=Y"
if /i "%~2"=="norun" set "RUN_AGENT=N"
if /i "%~3"=="norun" set "RUN_AGENT=N"

call :SaveLocalConfig

set "REMOTE_AGENT_DIR=%REMOTE_ROOT%/agent"
set "REMOTE_RUNTIME_DIR=%REMOTE_AGENT_DIR%/runtime"
set "REMOTE_CONFIG_PATH=%REMOTE_RUNTIME_DIR%/config.json"
set "REMOTE_BOOTSTRAP_PATH=%REMOTE_RUNTIME_DIR%/bootstrap.json"
set "REMOTE_ENTRY_PATH=%REMOTE_AGENT_DIR%/agent.js"
set "RUN_FILE=file://%REMOTE_ENTRY_PATH%"

if /i "%TARGET_ALL%"=="Y" (
    call :PushToAllDevices
    exit /b %ERRORLEVEL%
) else (
    call :PushToCurrentDevice
    exit /b %ERRORLEVEL%
)

:PushToCurrentDevice
echo.
echo 准备推送 Agent 到：%REMOTE_AGENT_DIR%
call :PushFilesToCurrentDevice
if errorlevel 1 exit /b 1

if /i "%RESET_CONFIG%"=="Y" (
    call :RunAdb shell rm -f "%REMOTE_CONFIG_PATH%"
    if errorlevel 1 exit /b 1
)

call :WriteBootstrapForCurrentDevice
if errorlevel 1 exit /b 1

if /i "%RUN_AGENT%"=="Y" (
    adb -s "%DEVICE%" shell am start -n "%AUTOJS_COMPONENT%" -d "%RUN_FILE%"
    if errorlevel 1 (
        echo AutoJs6 拉起失败，请手动打开：%REMOTE_ENTRY_PATH%
        exit /b 1
    )
) else (
    echo 已跳过自动运行，请手动打开：%REMOTE_ENTRY_PATH%
)

echo 推送完成。
exit /b 0

:PushToAllDevices
set "HAS_FAILURE=N"
for /l %%I in (1,1,!deviceCount!) do (
    set "DEVICE=!device[%%I]!"
    echo.
    echo ===========================
    echo 推送到设备：!DEVICE!
    echo ===========================
    call :PushFilesToCurrentDevice
    if errorlevel 1 set "HAS_FAILURE=Y"
    if not errorlevel 1 (
        if /i "%RESET_CONFIG%"=="Y" (
            call :RunAdb shell rm -f "%REMOTE_CONFIG_PATH%"
            if errorlevel 1 set "HAS_FAILURE=Y"
        )
        if not errorlevel 1 (
            call :WriteBootstrapForCurrentDevice
            if errorlevel 1 set "HAS_FAILURE=Y"
        )
        if not errorlevel 1 if /i "%RUN_AGENT%"=="Y" (
            adb -s "!DEVICE!" shell am start -n "%AUTOJS_COMPONENT%" -d "%RUN_FILE%"
            if errorlevel 1 set "HAS_FAILURE=Y"
        )
    )
)

if /i "%HAS_FAILURE%"=="Y" exit /b 1
echo 批量推送完成。
exit /b 0

:PushFilesToCurrentDevice
call :RunAdb shell mkdir -p "%REMOTE_AGENT_DIR%" "%REMOTE_RUNTIME_DIR%"
if errorlevel 1 exit /b 1
call :RunAdb push "%AGENT_ENTRY%" "%REMOTE_ENTRY_PATH%"
if errorlevel 1 exit /b 1
call :RunAdb push "%AGENT_LIB_DIR%" "%REMOTE_AGENT_DIR%/"
if errorlevel 1 exit /b 1
call :RunAdb push "%AGENT_SCRIPTS_DIR%" "%REMOTE_AGENT_DIR%/"
if errorlevel 1 exit /b 1
exit /b 0

:WriteBootstrapForCurrentDevice
set "TEMP_BOOTSTRAP=%TEMP%\mobilerpa-agent-bootstrap-%RANDOM%-%RANDOM%.json"
(
    echo {
    echo   "center_base_url": "%CENTER_BASE_URL%",
    echo   "websocket": {
    echo     "enabled": true,
    echo     "heartbeat_interval_ms": 30000
    echo   }
    echo }
) > "!TEMP_BOOTSTRAP!"
call :RunAdb push "!TEMP_BOOTSTRAP!" "%REMOTE_BOOTSTRAP_PATH%"
if errorlevel 1 (
    del /q "!TEMP_BOOTSTRAP!" >nul 2>nul
    exit /b 1
)
del /q "!TEMP_BOOTSTRAP!" >nul 2>nul
exit /b 0

:RunAdb
adb -s "%DEVICE%" %*
if errorlevel 1 exit /b 1
exit /b 0

:SaveLocalConfig
(
    echo DEVICE=%DEVICE%
    echo CENTER_BASE_URL=%CENTER_BASE_URL%
    echo AUTOJS_COMPONENT=%AUTOJS_COMPONENT%
) > "%CONFIG_FILE%" 2>nul
exit /b 0
