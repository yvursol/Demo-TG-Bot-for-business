@echo off
chcp 65001 > nul
echo ====================================
echo     NeuraMarket Bot (Deutsch)
echo ====================================

REM Zum Skriptverzeichnis wechseln
cd /d "%~dp0"

echo ✅ Abhängigkeiten werden installiert...
go mod tidy

echo 🚀 Bot wird gestartet...
go run bot.go

echo.
echo ⚠️  Wenn Fehler auftreten - prüfen Sie:
echo 1. Token in Umgebungsvariablen (TELEGRAM_BOT_TOKEN)
echo 2. Ihre Telegram-ID statt 123456789 (ADMIN_ID)
echo 3. Datenbankverbindung (DATABASE_URL)
echo.
pause
