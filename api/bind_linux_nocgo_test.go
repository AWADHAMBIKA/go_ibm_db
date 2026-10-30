//go:build linux && (amd64 || arm64) && !db2cgo

package api

import (
	"reflect"
	"testing"

	"github.com/ibmdb/go_ibm_db/internal/clidriver"
)

// Every entry point must be bound, including the integer-valued and
// pointer-valued variants of SQLSetEnvAttr and SQLSetConnectAttrW. They are
// separate Go function values over the same C symbol, so neither can overwrite
// the other.
func TestAllEntryPointsBound(t *testing.T) {
	if clidriver.Resolve().Lib == "" {
		t.Skip("no Db2 CLI driver installed; run the installer first")
	}
	if !ensureLoaded() {
		t.Fatalf("ensureLoaded failed: %v", LoadError())
	}
	for name, fn := range map[string]any{
		"SQLAllocHandle":        fnSQLAllocHandle,
		"SQLDriverConnectW":     fnSQLDriverConnectW,
		"SQLSetEnvAttr(int)":    fnSQLSetEnvAttr,
		"SQLSetEnvAttr(ptr)":    fnSQLSetEnvAttrPtr,
		"SQLSetConnectAttrW(i)": fnSQLSetConnectAttrW,
		"SQLSetConnectAttrW(p)": fnSQLSetConnAttrWPtr,
		"SQLSetStmtAttrW":       fnSQLSetStmtAttrW,
		"SQLExecDirectW":        fnSQLExecDirectW,
	} {
		if reflect.ValueOf(fn).IsNil() {
			t.Errorf("%s is not bound", name)
		}
	}
}
