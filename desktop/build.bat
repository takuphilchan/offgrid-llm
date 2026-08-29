@echo off
setlocal

set "DESKTOP_DIR=%~dp0"
for %%I in ("%DESKTOP_DIR%..") do set "PROJECT_ROOT=%%~fI"
set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=current"

pushd "%PROJECT_ROOT%\web\app" || exit /b 1
call npm ci || exit /b 1
call npm run api:check || exit /b 1
call npm run check || exit /b 1
call npm run build || exit /b 1
popd

pushd "%DESKTOP_DIR%" || exit /b 1
call npm ci || exit /b 1

if "%TARGET%"=="win" (
  call npm run build:win
) else if "%TARGET%"=="current" (
  call npm run build
) else (
  echo Usage: build.bat [current^|win] 1>&2
  exit /b 2
)

if errorlevel 1 exit /b 1
echo Desktop artifacts are in %DESKTOP_DIR%dist
popd
endlocal
