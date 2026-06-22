@echo off
chcp 65001 >nul
setlocal EnableExtensions

if "%~1"=="" (
    echo Usage: push-agent-center.bat ^<device^> ^<center_base_url^> [reset] [norun]
    exit /b 1
)

if "%~2"=="" (
    echo Missing center base URL.
    exit /b 1
)

set "DEVICE=%~1"
set "CENTER_BASE_URL=%~2"
set "RESET_CONFIG=N"
set "RUN_AGENT=Y"

if /i "%~3"=="reset" set "RESET_CONFIG=Y"
if /i "%~4"=="reset" set "RESET_CONFIG=Y"
if /i "%~3"=="norun" set "RUN_AGENT=N"
if /i "%~4"=="norun" set "RUN_AGENT=N"

for %%I in ("%~dp0..\..") do set "REPO_ROOT=%%~fI"
set "AGENT_DIR=%REPO_ROOT%\agent"
set "AGENT_ENTRY=%AGENT_DIR%\agent.js"
set "AGENT_LIB_DIR=%AGENT_DIR%\lib"
set "AGENT_SCRIPTS_DIR=%AGENT_DIR%\scripts"
set "REMOTE_ROOT=/sdcard/脚本"
set "REMOTE_AGENT_DIR=%REMOTE_ROOT%/agent"
set "REMOTE_RUNTIME_DIR=%REMOTE_AGENT_DIR%/runtime"
set "REMOTE_CONFIG_PATH=%REMOTE_RUNTIME_DIR%/config.json"
set "REMOTE_BOOTSTRAP_PATH=%REMOTE_RUNTIME_DIR%/bootstrap.json"
set "REMOTE_ENTRY_PATH=%REMOTE_AGENT_DIR%/agent.js"
set "RUN_FILE=file://%REMOTE_ENTRY_PATH%"
set "AUTOJS_COMPONENT=org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity"

if not exist "%AGENT_ENTRY%" (
    echo Agent entry not found: %AGENT_ENTRY%
    exit /b 1
)
if not exist "%AGENT_LIB_DIR%\" (
    echo Agent lib dir not found: %AGENT_LIB_DIR%
    exit /b 1
)
if not exist "%AGENT_SCRIPTS_DIR%\" (
    echo Agent scripts dir not found: %AGENT_SCRIPTS_DIR%
    exit /b 1
)

adb connect "%DEVICE%" >nul 2>nul

call :RunDeviceAdb shell mkdir -p "%REMOTE_AGENT_DIR%" "%REMOTE_RUNTIME_DIR%"
if errorlevel 1 exit /b 1
call :RunDeviceAdb push "%AGENT_ENTRY%" "%REMOTE_ENTRY_PATH%"
if errorlevel 1 exit /b 1
call :RunDeviceAdb push "%AGENT_LIB_DIR%" "%REMOTE_AGENT_DIR%/"
if errorlevel 1 exit /b 1
call :RunDeviceAdb push "%AGENT_SCRIPTS_DIR%" "%REMOTE_AGENT_DIR%/"
if errorlevel 1 exit /b 1

if /i "%RESET_CONFIG%"=="Y" (
    call :RunDeviceAdb shell rm -f "%REMOTE_CONFIG_PATH%"
    if errorlevel 1 exit /b 1
)

call :WriteBootstrap
if errorlevel 1 exit /b 1

if /i "%RUN_AGENT%"=="Y" (
    adb -s "%DEVICE%" shell am start -n "%AUTOJS_COMPONENT%" -d "%RUN_FILE%"
    if errorlevel 1 (
        echo AutoJs6 start failed. Open manually: %REMOTE_ENTRY_PATH%
        exit /b 1
    )
) else (
    echo Skip auto run. Open manually in AutoJs6: %REMOTE_ENTRY_PATH%
)

echo Agent deploy finished: %DEVICE%
exit /b 0

:WriteBootstrap
set "TEMP_BOOTSTRAP=%TEMP%\mobilerpa-agent-bootstrap-%RANDOM%-%RANDOM%.json"
(
    echo {
    echo   "center_base_url": "%CENTER_BASE_URL%",
    echo   "websocket": {
    echo     "enabled": true,
    echo     "heartbeat_interval_ms": 30000
    echo   }
    echo }
) > "%TEMP_BOOTSTRAP%"
call :RunDeviceAdb push "%TEMP_BOOTSTRAP%" "%REMOTE_BOOTSTRAP_PATH%"
set "PUSH_RESULT=%ERRORLEVEL%"
del /q "%TEMP_BOOTSTRAP%" >nul 2>nul
exit /b %PUSH_RESULT%

:RunDeviceAdb
adb -s "%DEVICE%" %*
if errorlevel 1 exit /b 1
exit /b 0
