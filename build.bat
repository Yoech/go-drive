@echo off
set BUILD_VERSION=%1
echo %BUILD_VERSION%

for /f "delims=" %%i in ('git rev-parse HEAD') do set REV_HASH=%%i
echo %REV_HASH%

for /f "delims=" %%i in ('date /T') do set TIMESTAMP=%%i
echo %TIMESTAMP%

set BUILD_DIR=.\build\go-drive_build\
echo %BUILD_DIR%

set CGO_CFLAGS=-Wno-return-local-addr
echo %CGO_CFLAGS%

go build -o "%BUILD_DIR%\go-drive.exe" -ldflags "-w -s -X 'go-drive/common.Version=%BUILD_VERSION%' -X 'go-drive/common.RevHash=%REV_HASH%' -X 'go-drive/common.BuildAt=%TIMESTAMP%'"