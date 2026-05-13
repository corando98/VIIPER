@echo off
set PATH=C:\w64devkit\w64devkit\bin;%PATH%
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
cd /d "%~dp0"
"C:\Program Files\Go\bin\go.exe" build -buildmode=c-shared -o libviiper.dll ./clib/
if %ERRORLEVEL% EQU 0 (
    echo BUILD SUCCESS: libviiper.dll created
    copy /Y libviiper.dll "..\src\ViiperController\libviiper.dll" >nul 2>&1
    if %ERRORLEVEL% EQU 0 (
        echo Copied to ViiperController project directory
    )
) else (
    echo BUILD FAILED with error code %ERRORLEVEL%
)
