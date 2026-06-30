@echo off
cd /d "%~dp0"
if exist dify-log-excel.exe goto run
cd ..
:run
dify-log-excel.exe serve
pause
