:: Optional helper: points the current command prompt at an existing Db2 CLI driver
:: and puts its bin folder on PATH so db2cli.exe can be run.
:: It is NOT required for go_ibm_db. By default the driver downloads the CLI driver
:: to %USERPROFILE%\.ibm_db\clidriver on first connection and loads it by absolute
:: path. PATH, LIB and CGO_* settings are not needed on Windows.
::
:: Usage: setenvwin.bat [path-to-clidriver]
:: Without an argument it uses %USERPROFILE%\.ibm_db\clidriver.

@echo off
if "%~1"=="" (
    set "IBM_DB_HOME=%USERPROFILE%\.ibm_db\clidriver"
) else (
    set "IBM_DB_HOME=%~f1"
)
set "PATH=%IBM_DB_HOME%\bin;%PATH%"
echo IBM_DB_HOME=%IBM_DB_HOME%

