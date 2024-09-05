chcp 936
@echo off
SETLOCAL EnableDelayedExpansion
for /F "tokens=1,2 delims=#" %%a in ('"prompt #$H#$E# & echo on & for %%b in (1) do rem"') do (
  set "DEL=%%a"
)

call:printGoEnv
call:runtime

:printGoEnv
echo *********************************
go env
echo *********************************
for /f "delims=" %%v in ('go env ^| findstr GOVERSION') do set goVersion=%%v
for /f "delims=" %%v in ('go env ^| findstr GOPROXY') do set goProxy=%%v
for /f "delims=" %%v in ('go env ^| findstr GOOS') do set goGoos=%%v
for /f "delims=" %%v in ('go env ^| findstr GOHOSTARCH') do set goHostArch=%%v
call:Color 0a "%goVersion%"
call:Color 0a "%goProxy%"
call:Color 0a "%goGoos%"
call:Color 0a "%goHostArch%"
echo *********************************
goto:eof

:Color
@echo off
set val=%~2
:: http:// =>
set val=%val:http://=%
:: https:// =>
set val=%val:https://=%
:: / =>
::set val=%val:/=%
:: set =>
::set val=%val:set =%
<nul set /p ".=%DEL%" > "%val%"
findstr /v /a:%1 /R "^$" "%val%" nul
del "%val%" > nul 2>&1
echo.
goto:eof

:runtime
cd .\\build\\go-drive_build
start go-drive.exe -c=config.yml
cd ..\\..\\
goto:eof