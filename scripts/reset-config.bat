@echo off
:: Deletes the filehub config and manifests so the app starts fresh.
:: Run this when the app is not running.

set "dir=%APPDATA%\filehub"

if not exist "%dir%" (
    echo Nothing to delete — %dir% does not exist.
    exit /b 0
)

echo Deleting: %dir%
rmdir /s /q "%dir%"
echo Done. filehub will start fresh on next launch.
