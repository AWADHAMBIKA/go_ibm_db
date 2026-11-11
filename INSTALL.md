# Installing go_ibm_db

*Copyright (c) 2014 IBM Corporation and/or its affiliates. All rights reserved.*

Permission is hereby granted, free of charge, to any person obtaining a copy of this
software and associated documentation files (the "Software"), to deal in the Software
without restriction, including without limitation the rights to use, copy, modify, 
merge, publish, distribute, sublicense, and/or sell copies of the Software, 
and to permit persons to whom the Software is furnished to do so, subject to the 
following conditions:

The above copyright notice and this permission notice shall be included in all copies 
or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, 
INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR 
PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE 
FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR 
OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER 
DEALINGS IN THE SOFTWARE.

## Contents

1. [Overview](#Installation)
2. [ibm_db Installation on Linux](#inslnx)
3. [ibm_db Installation on MacOS](#insmac)
4. [ibm_db Installation on Windows](#inswin)

## <a name="overview"></a> 1. Overview

The [*go_ibm_db*](https://github.com/ibmdb/go_ibm_db) is an asynchronous/synchronous interface for GoLang to IBM DB2.

Following are the steps to installation in your system.

This go_ibm_db driver has been tested on 64-bit/32-bit IBM Linux, MacOS and Windows.

### 1.1 clidriver info for MacOS

* Latest version of clidriver available for MacOS x64 system is: v11.5.9
* By default on Intel Chip Macos, clidriver of v11.5.9 will get downloaded; its GCC 8 runtime dependencies are not bundled.
* First version of clidriver supported on MacOS ARM64 system is: v12.1.0
* On MacOS Apple Silicon, the default is v12.1.4.

### 1.2 License requirement to connect to Db2 for z/OS and Db2 for iSeries servers

* Please read [this doc](https://github.com/ibmdb/go_ibm_db/blob/master/README.md#for-zos-and-iseries-connectivity-and-sql1598n-error) for detail info about license requiremnet and resolving SQL1598N error during connection.
* clidriver v12.1.0 requires db2connect v12.1 license to connect z/OS or iSeries severs.
* You can force go_ibm_db driver to use older version of clidirver by setting system level environment varialbe CLIDRIVER_DOWNLOAD_VERSION or explicitly setting IBM_DB_DOWNLOAD_URL to point path of clidriver.tar.gz file.
```
    export CLIDRIVER_DOWNLOAD_VERSION=v11.5.9
    export IBM_DB_DOWNLOAD_URL=https://public.dhe.ibm.com/ibmdl/export/pub/software/data/db2/drivers/odbc_cli/v11.5.9/linuxx64_odbc_cli.tar.gz
    go run setup.go
```
(On Linux use `go run github.com/ibmdb/go_ibm_db/installer@latest` instead of `go run setup.go`.)

## <a name="inslnx"></a> 2. Go_ibm_db Installation on Linux.

### 2.1 Install GoLang for Linux

Download the
[GoLang Linux binaries](https://golang.org/dl) or [Go Latest binaries](https://go.dev/dl) and
extract the file, for example into `$HOME/mygo`:

```
cd $HOME/mygo
wget -c https://golang.org/dl/go1.22.1.linux-amd64.tar.gz
tar -xzf go1.22.1.linux-amd64.tar.gz
export GOROOT=$HOME/mygo/go
export GOPATH=$HOME/mygo
```

Set PATH to include Go:

```
export PATH=/mygo/go/bin:$PATH
```

### 2.2 Install go_ibm_db (Linux amd64 and arm64: two steps, no C compiler)

On Linux amd64 and arm64 the driver loads the Db2 CLI library at run time, so GCC, `CGO_CFLAGS`, `CGO_LDFLAGS` and `LD_LIBRARY_PATH` are not needed and `CGO_ENABLED=0` works.

```
1. go get github.com/ibmdb/go_ibm_db
2. go run .
```

`go run .` runs your own application: a `main` package that imports `_ "github.com/ibmdb/go_ibm_db"` and calls `sql.Open("go_ibm_db", dsn)`.

* On the first connection the driver looks for the CLI driver in `IBM_DB_HOME`, then in `~/.ibm_db/clidriver`; if neither has it, it downloads it (HTTPS, amd64 only) into `~/.ibm_db/clidriver`.
* To provision ahead of time (containers, CI, air-gapped build stages): `go run github.com/ibmdb/go_ibm_db/installer@latest`. Use `-verify` to see which CLI driver will be used and `-dir <path>` to choose the target directory.
* On arm64 there is no automatic download because IBM publishes no Linux arm64 CLI archive; set `IBM_DB_HOME` to an existing CLI driver.
* Environment variables: `IBM_DB_HOME` (use an existing CLI driver), `IBM_DB_AUTO_DOWNLOAD=0` (never download), `IBM_DB_DOWNLOAD_URL` (HTTPS archive URL), `IBM_DB_DOWNLOAD_SHA256` (optional checksum), `CLIDRIVER_DOWNLOAD_VERSION` (pin a version), `DOWNLOAD_CLIDRIVER=true` (installer only: download again even if a CLI driver exists). `IBM_DB_INSTALLER_URL` and `DB2HOME` are deprecated aliases.
* If the library is not found, the error lists every searched path and how to fix it.
* The CLI driver needs a recent glibc (2.34 or newer for current releases).

Linux on ppc64le and s390x, and builds using `-tags db2cgo`, still use cgo and the manual steps below.

#### Manual steps (cgo builds: ppc64le, s390x, `-tags db2cgo`)

Following are the steps to install [*go_ibm_db*](https://github.com/ibmdb/go_ibm_db) from github.
using directory `/goapp` for example.

#### 2.2.1 Direct Installation.
```
1. mkdir goapp
2. cd goapp
3. go install github.com/ibmdb/go_ibm_db/installer@latest
   or
   go install github.com/ibmdb/go_ibm_db/installer@v0.5.2
4. ls $GOPATH/pkg/mod/github.com/ibmdb
   go_ibm_db@v0.5.2
5. cd $GOPATH/pkg/mod/github.com/ibmdb/go_ibm_db@v0.5.2/installer
6. go run ./setup.go
7. export IBM_DB_HOME=$GOPATH/pkg/mod/github.com/ibmdb/clidriver
8. source ./setenv.sh
```

It's Done.

#### 2.2.2 Manual Installation by using git clone.

```
1. mkdir $HOME/goapp
2. cd goapp
3. git clone https://github.com/ibmdb/go_ibm_db/
4. go env GOPATH
5. cd go_ibm_db/installer
6. go run ./setup.go
7. export IBM_DB_HOME=$HOME/goapp/clidriver
8. source ./setenv.sh
```

If IBM_DB_HOME is already set or, sourcing setenv.sh fails, create below environment variables:
```
export CGO_CFLAGS=-I$IBM_DB_HOME/include
export CGO_LDFLAGS=-L$IBM_DB_HOME/lib 
export LD_LIBRARY_PATH=$IBM_DB_HOME/lib:$LD_LIBRARY_PATH
```

## <a name="insmac"></a> 3. Go_ibm_db Installation on MacOS x64 and arm64 Systems

### 3.1 Install GoLang for Mac

Download the
[GoLang MacOS binaries](https://golang.org/dl) or [GoLang Latest binaries](https://go.dev/dl) and
extract the file.

### 3.2 Install go_ibm_db

In your Go application directory:

```sh
go get github.com/ibmdb/go_ibm_db
go run .
```

On Apple Silicon this requires no C compiler, Xcode command-line tools, `CGO_*` variables, or `DYLD_LIBRARY_PATH`; `CGO_ENABLED=0` is supported. On first connection the driver downloads and verifies the CLI driver into `~/.ibm_db/clidriver`, unless a valid `IBM_DB_HOME` supplies an existing installation.

The Intel v11.5.9 CLI dylib depends on GCC 8 runtime libraries under `/usr/local/lib/gcc/8/` that are not included in IBM's archive. A clean Intel system may therefore fail to load the library; use a compatible installation via `IBM_DB_HOME` or provide its Intel runtime dependencies. Native Intel testing is still required.

### 3.3 Configuration

* `IBM_DB_HOME`: use an existing directory containing `lib/libdb2.dylib`.
* `IBM_DB_AUTO_DOWNLOAD=0`: disable automatic downloads.
* `DOWNLOAD_CLIDRIVER=true`: installer only; download again into `~/.ibm_db/clidriver` (or `-dir`) even if a CLI driver exists.
* `IBM_DB_DOWNLOAD_URL`: use an HTTPS archive URL.
* `CLIDRIVER_DOWNLOAD_VERSION`: select a CLI version. The defaults are v12.1.4 for Apple Silicon and v11.5.9 for Intel.
* `IBM_DB_DOWNLOAD_SHA256`: required for a custom URL or a non-default version; supply a trusted archive digest.

Default archives are pinned and checked against SHA-256 values recorded by this project. Provision in advance with `go run github.com/ibmdb/go_ibm_db/installer@latest` and inspect the resolved path with `go run github.com/ibmdb/go_ibm_db/installer@latest -verify`. The `-tags db2cgo` fallback still requires the CLI headers, a C compiler and the usual cgo configuration. Do not disable System Integrity Protection.

## <a name="inswin"></a> 4. Go_ibm_db Installation on Windows.

### 4.1 Install GoLang for Windows

Download the [Go Windows binary/installer](https://golang.org/dl) or [Go Latest binaries](https://go.dev/dl/) and
install it.

### 4.2 Install go_ibm_db (Windows amd64 and 386: two steps, no C compiler)

On Windows the driver loads the Db2 CLI DLL (`db2cli64.dll`, or `db2cli.dll` on 32-bit) at run time by absolute path, so GCC, `PATH`, `LIB`, `CGO_CFLAGS` and `CGO_LDFLAGS` are not needed and `CGO_ENABLED=0` works.

```
1. go get github.com/ibmdb/go_ibm_db
2. go run .
```

`go run .` runs your own application: a `main` package that imports `_ "github.com/ibmdb/go_ibm_db"` and calls `sql.Open("go_ibm_db", dsn)`.

* On the first connection the driver looks for the CLI driver in `IBM_DB_HOME`, then in `%USERPROFILE%\.ibm_db\clidriver`, then on `PATH`; if none has it, it downloads it (HTTPS) into `%USERPROFILE%\.ibm_db\clidriver`.
* To provision ahead of time (CI, air-gapped build stages): `go run github.com/ibmdb/go_ibm_db/installer@latest`. Use `-verify` to see which CLI driver will be used and `-dir <path>` to choose the target directory.
* Windows on ARM64 has no published CLI driver; set `IBM_DB_HOME` to an existing one.
* To use an existing CLI driver, for example `set IBM_DB_HOME=C:\clidriver`. Runtime Client (RTCL) packages work too because no `include` directory is needed.
* Environment variables: `IBM_DB_HOME` (use an existing CLI driver), `IBM_DB_AUTO_DOWNLOAD=0` (never download), `IBM_DB_DOWNLOAD_URL` (HTTPS `.zip` URL), `IBM_DB_DOWNLOAD_SHA256` (optional checksum), `CLIDRIVER_DOWNLOAD_VERSION` (pin a version), `DOWNLOAD_CLIDRIVER=true` (installer only: download again even if a CLI driver exists). `IBM_DB_INSTALLER_URL` and `DB2HOME` are deprecated aliases.
* If the DLL is not found, the error lists every searched path and how to fix it.

### 4.3 Optional helper script

`installer\setenvwin.bat [path-to-clidriver]` sets `IBM_DB_HOME` and adds its `bin` folder to `PATH` for the current prompt, for example to run `db2cli.exe`. It is not needed to build or run go_ibm_db.


4. Download platform specific clidriver from https://public.dhe.ibm.com/ibmdl/export/pub/software/data/db2/drivers/odbc_cli/ , untar/unzip it and set `IBM_DB_HOME` environmental variable to full path of extracted 'clidriver' directory, for example if clidriver is extracted as: `/home/mysystem/clidriver`, then set system level environment variable `IBM_DB_HOME=/home/mysystem/clidriver`.

