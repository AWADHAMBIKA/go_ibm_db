package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ibmdb/go_ibm_db/internal/clidriver"
)

func TestLoadAbsRejectsRelativePath(t *testing.T) {
	if _, err := loadAbs("db2cli64.dll"); err == nil {
		t.Fatal("expected relative DLL path to be rejected")
	}
}

func TestFindOnPathIgnoresRelativeDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.dll"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "."+string(os.PathListSeparator)+dir)
	if got := findOnPath("x.dll"); got != filepath.Join(dir, "x.dll") {
		t.Fatalf("got %q", got)
	}
	t.Setenv("PATH", ".")
	if got := findOnPath("x.dll"); got != "" {
		t.Fatalf("relative PATH entry must be ignored, got %q", got)
	}
}

// Loads the real CLI DLL when one is installed, without touching the network.
func TestLoadInstalledDLL(t *testing.T) {
	if clidriver.Resolve().Lib == "" {
		t.Skip("no Db2 CLI driver installed; run the installer first")
	}
	if !ensureLoaded() {
		t.Fatalf("ensureLoaded failed: %v", LoadError())
	}
	var env SQLHANDLE
	if ret := SQLAllocHandle(SQL_HANDLE_ENV, SQLHANDLE(SQL_NULL_HANDLE), &env); ret != SQL_SUCCESS {
		t.Fatalf("SQLAllocHandle returned %d", ret)
	}
	SQLFreeHandle(SQL_HANDLE_ENV, env)
}
