//go:build windows || (linux && (amd64 || arm64) && !db2cgo)

package go_ibm_db

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/ibmdb/go_ibm_db/api"
	"github.com/ibmdb/go_ibm_db/internal/clidriver"
)

// isolateCLIDriverLookup hides every place the CLI driver could be found.
func isolateCLIDriverLookup(t *testing.T) string {
	t.Helper()
	// Not t.TempDir(): a loaded DLL stays locked on Windows, so cleanup is best effort.
	home, err := os.MkdirTemp("", "go_ibm_db_home")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(clidriver.EnvHome, "")
	t.Setenv(clidriver.EnvLegacyHome, "")
	t.Setenv("PATH", t.TempDir())
	return home
}

func TestOpenReportsMissingCLIDriver(t *testing.T) {
	isolateCLIDriverLookup(t)
	t.Setenv(clidriver.EnvAutoDownload, "0")

	db, err := sql.Open("go_ibm_db", "HOSTNAME=127.0.0.1;PORT=1;DATABASE=x;UID=u;PWD=p")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err == nil {
		t.Fatal("expected an error when the CLI driver is unavailable")
	}
	if api.LoadError() == nil {
		t.Skipf("CLI driver is available on this machine (error was: %v)", err)
	}
	for _, want := range []string{"not found", "Searched", clidriver.EnvHome} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// Set IBM_DB_E2E_DOWNLOAD=1 to run the real first-connection download from IBM.
func TestFirstConnectionProvisionsCLIDriver(t *testing.T) {
	if os.Getenv("IBM_DB_E2E_DOWNLOAD") != "1" {
		t.Skip("set IBM_DB_E2E_DOWNLOAD=1 to download the real CLI driver")
	}
	home := isolateCLIDriverLookup(t)
	t.Setenv(clidriver.EnvAutoDownload, "")

	db, err := sql.Open("go_ibm_db", "HOSTNAME=127.0.0.1;PORT=1;DATABASE=x;UID=u;PWD=p")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The host is unreachable, so the connection must fail inside Db2 CLI,
	// which proves the library was downloaded, loaded and called.
	err = db.Ping()
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if lerr := api.LoadError(); lerr != nil {
		t.Fatalf("CLI driver did not load: %v", lerr)
	}
	if !strings.Contains(err.Error(), "SQLDriverConnect") {
		t.Fatalf("expected a SQLDriverConnect error from Db2 CLI, got: %v", err)
	}
	def, _ := clidriver.DefaultDir()
	if !clidriver.Valid(def) {
		t.Fatalf("CLI driver was not installed under %s (home %s)", def, home)
	}
}
