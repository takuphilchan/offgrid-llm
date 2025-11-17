@echo off
REM Build script for OffGrid LLM Desktop on Windows

cd /d "%~dp0"

echo Building OffGrid LLM Desktop Applications...
echo.

REM Check if node_modules exists
if not exist "node_modules\" (
    echo Installing dependencies...
    call npm install
)

REM Clean previous builds
echo Cleaning previous builds...
if exist "dist\" (
    rmdir /s /q dist
)

REM Build
if "%1"=="all" (
    echo Building for all platforms...
    call npm run build:all
) else if "%1"=="win" (
    echo Building for Windows...
    call npm run build:win
) else (
    echo Building for current platform...
    call npm run build
)

echo.
echo Build complete! Installers are in desktop\dist\
dir dist\
pause
