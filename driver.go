// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package odbc implements database/sql driver to access data via odbc interface.
package go_ibm_db

import (
	"database/sql"
	"os"
	"sync"

	"github.com/ibmdb/go_ibm_db/api"
	trc "github.com/ibmdb/go_ibm_db/log2"
)

var drv Driver

type Driver struct {
	Stats
	h api.SQLHENV // environment handle
}

func initDriver() error {
	trc.Trace1("driver.go:InitDriver() - ENTRY")

	//Allocate environment handle
	var out api.SQLHANDLE
	in := api.SQLHANDLE(api.SQL_NULL_HANDLE)
	ret := api.SQLAllocHandle(api.SQL_HANDLE_ENV, in, &out)
	if IsError(ret) {
		if err := api.LoadError(); err != nil {
			return err
		}
		return NewError("SQLAllocHandle", api.SQLHENV(in))
	}
	h := api.SQLHENV(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_ENV, 1)

	// will use ODBC v3
	ret = api.SQLSetEnvUIntPtrAttr(h, api.SQL_ATTR_ODBC_VERSION,
		api.SQL_OV_ODBC3, 0)
	if IsError(ret) {
		defer releaseHandle(h)
		return NewError("SQLSetEnvAttr ODBC v3", h)
	}
	drv.h = h

	trc.Trace1("driver.go:InitDriver() - EXIT")
	return nil
}

var drvInitMu sync.Mutex

// ensureInit allocates the environment handle on first use so that the Db2 CLI
// driver is located (and provisioned if needed) on the first connection, not at import.
func (d *Driver) ensureInit() error {
	drvInitMu.Lock()
	defer drvInitMu.Unlock()
	if d.h != api.SQLHENV(api.SQL_NULL_HENV) {
		return nil
	}
	return initDriver()
}

func (d *Driver) Close() error {
	trc.Trace1("driver.go: Close() - ENTRY")

	// TODO(brainman): who will call (*Driver).Close (to dispose all opened handles)?
	h := d.h
	d.h = api.SQLHENV(api.SQL_NULL_HENV)

	trc.Trace1("driver.go: Close() - EXIT")
	return releaseHandle(h)
}

func init() {
	trc.GetPath(os.Getenv("GO_IBMDB_TRACE"))

	trc.Trace1("driver.go:init() - ENTRY")

	//go's to databse/sql/sql.go 43 line
	sql.Register("go_ibm_db", &drv)

	trc.Trace1("driver.go:init() - EXIT")
}
