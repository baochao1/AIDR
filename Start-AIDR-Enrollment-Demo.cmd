@echo off
setlocal
cd /d "%~dp0"
go build -o build\aidr-enrollment-demo.exe ./cmd/supervisor-demo
if errorlevel 1 goto failed
python scripts\aidr-enrollment-demo.py build\aidr-enrollment-demo.exe --present
if errorlevel 1 goto failed
pause
exit /b 0
:failed
echo AIDR enrollment demo failed.
pause
exit /b 1
