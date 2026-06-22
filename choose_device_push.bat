@echo off
setlocal EnableDelayedExpansion

set "CONFIG_FILE=%~dp0config.ini"
set "PROJECTS_DIR=%~dp0projects"
set "SCRIPTS_DIR=%~dp0scripts"
set "REMOTE_DIR=/sdcard/scripts"

set "LAST_DEVICE="
set "LAST_TARGET="
set "LAST_TARGET_TYPE="
if exist "%CONFIG_FILE%" (
    for /f "tokens=1,* delims==" %%A in (%CONFIG_FILE%) do (
        if /i "%%A"=="DEVICE" set "LAST_DEVICE=%%B"
        if /i "%%A"=="TARGET" set "LAST_TARGET=%%B"
        if /i "%%A"=="TARGET_TYPE" set "LAST_TARGET_TYPE=%%B"
    )
)

if not defined LAST_TARGET_TYPE (
    if defined LAST_TARGET (
        if exist "%PROJECTS_DIR%\%LAST_TARGET%\main.js" (
            set "LAST_TARGET_TYPE=DIR"
        ) else if exist "%SCRIPTS_DIR%\%LAST_TARGET%" (
            set "LAST_TARGET_TYPE=FILE"
        )
    )
)
if not defined LAST_TARGET_TYPE set "LAST_TARGET_TYPE=DIR"

set "deviceCount=0"
echo ===========================
echo Available devices:
echo ===========================
for /f "skip=1 tokens=1" %%A in ('adb devices') do (
    if not "%%A"=="" (
        set /a deviceCount+=1
        set "device[!deviceCount!]=%%A"
        echo !deviceCount!. %%A
    )
)
if %deviceCount%==0 (
    echo No devices found.
    exit /b 1
)

set "DEFAULT_DEVICE=!device[1]!"
for /l %%I in (1,1,%deviceCount%) do (
    if /i "!device[%%I]!"=="%LAST_DEVICE%" set "DEFAULT_DEVICE=%LAST_DEVICE%"
)

echo.
set /p "choice=Choose device number (default %DEFAULT_DEVICE%): "
set "choiceRaw=%choice%"
for /f "tokens=* delims= " %%A in ("%choiceRaw%") do set "choice=%%A"
if not defined choice (
    set "DEVICE=%DEFAULT_DEVICE%"
) else (
    for /f "tokens=1" %%A in ("!choice!") do set "choice=%%A"
    if defined device[!choice!] (
        set "DEVICE=!device[!choice!]!"
    ) else (
        set "DEVICE=!choice!"
    )
)
set "LAST_DEVICE=%DEVICE%"
echo Selected device: %DEVICE%

set "itemCount=0"
echo.
echo ===========================
echo Projects:
echo ===========================
for /f "delims=" %%D in ('dir /b /ad /on "%PROJECTS_DIR%" 2^>nul') do (
    if exist "%PROJECTS_DIR%\%%D\main.js" (
        set /a itemCount+=1
        set "ITEM[!itemCount!]=%%D"
        set "TYPE[!itemCount!]=DIR"
        echo !itemCount!. %%D [DIR]
    )
)

echo.
echo ===========================
echo Scripts:
echo ===========================
for /f "delims=" %%F in ('dir /b /a-d /on "%SCRIPTS_DIR%\*.js" 2^>nul') do (
    set /a itemCount+=1
    set "ITEM[!itemCount!]=%%F"
    set "TYPE[!itemCount!]=FILE"
    echo !itemCount!. %%F [FILE]
)

if %itemCount%==0 (
    echo No project or script found.
    exit /b 1
)

if defined LAST_TARGET (
    set "DEFAULT_TARGET=%LAST_TARGET%"
) else (
    set "DEFAULT_TARGET=!ITEM[1]!"
)

echo.
set /p "choiceTarget=Choose project/script number (default %DEFAULT_TARGET%): "
set "choiceTargetRaw=%choiceTarget%"
for /f "tokens=* delims= " %%A in ("%choiceTargetRaw%") do set "choiceTarget=%%A"
if not defined choiceTarget (
    set "TARGET_NAME=%LAST_TARGET%"
    set "TARGET_TYPE=%LAST_TARGET_TYPE%"
) else (
    for /f "tokens=1" %%A in ("!choiceTarget!") do set "choiceTarget=%%A"
    if defined ITEM[!choiceTarget!] (
        set "TARGET_NAME=!ITEM[!choiceTarget!]!"
        set "TARGET_TYPE=!TYPE[!choiceTarget!]!"
    ) else (
        set "TARGET_NAME=!choiceTarget!"
        set "TARGET_TYPE="
        if exist "%PROJECTS_DIR%\!choiceTarget!\main.js" set "TARGET_TYPE=DIR"
        if not defined TARGET_TYPE if exist "%SCRIPTS_DIR%\!choiceTarget!" set "TARGET_TYPE=FILE"
        if not defined TARGET_TYPE set "TARGET_TYPE=FILE"
    )
)

set "LAST_TARGET=%TARGET_NAME%"
set "LAST_TARGET_TYPE=%TARGET_TYPE%"
echo Selected target: %TARGET_NAME%
echo Type: %TARGET_TYPE%

(
    echo DEVICE=%LAST_DEVICE%
    echo TARGET=%LAST_TARGET%
    echo TARGET_TYPE=%LAST_TARGET_TYPE%
) > "%CONFIG_FILE%"

if /i "%TARGET_TYPE%"=="DIR" (
    echo.
    echo Pushing project %TARGET_NAME% ...
    adb -s %DEVICE% shell mkdir -p "%REMOTE_DIR%/projects"
    adb -s %DEVICE% shell rm -rf "%REMOTE_DIR%/projects/%TARGET_NAME%"
    adb -s %DEVICE% push "%PROJECTS_DIR%\%TARGET_NAME%" "%REMOTE_DIR%/projects/"
    set "RUN_FILE=file://%REMOTE_DIR%/projects/%TARGET_NAME%/main.js"
) else (
    echo.
    echo Pushing script %TARGET_NAME% ...
    adb -s %DEVICE% shell mkdir -p "%REMOTE_DIR%/scripts"
    adb -s %DEVICE% push "%SCRIPTS_DIR%\%TARGET_NAME%" "%REMOTE_DIR%/scripts/"
    set "RUN_FILE=file://%REMOTE_DIR%/scripts/%TARGET_NAME%"
)

echo.
echo Running...
adb -s %DEVICE% shell am start -n org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity -d %RUN_FILE%

echo.
echo Done.
exit /b 0
