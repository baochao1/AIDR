@echo off
cd /d "%~dp0"
python scripts\aidr-agent-console.py
if errorlevel 1 pause
