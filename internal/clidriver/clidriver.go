// Package clidriver locates, downloads, verifies and extracts the Db2 CLI driver.
// It uses only the Go standard library and never writes to the Go module cache.
package clidriver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvHome            = "IBM_DB_HOME"
	EnvLegacyHome      = "DB2HOME"
	EnvAutoDownload    = "IBM_DB_AUTO_DOWNLOAD"
	EnvDownloadURL     = "IBM_DB_DOWNLOAD_URL"
	EnvLegacyURL       = "IBM_DB_INSTALLER_URL"
	EnvDownloadSHA256  = "IBM_DB_DOWNLOAD_SHA256"
	EnvDownloadVersion = "CLIDRIVER_DOWNLOAD_VERSION"
)

// Warn receives deprecation and progress messages. Tests may replace it.
var Warn = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "go_ibm_db: "+format+"\n", args...)
}

// DefaultDir returns ~/.ibm_db/clidriver.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".ibm_db", "clidriver"), nil
}

// LibName returns the CLI library file name for the current platform.
func LibName() string {
	return libNameFor(runtime.GOOS, runtime.GOARCH)
}

func libNameFor(goos, goarch string) string {
	switch goos {
	case "darwin":
		return "libdb2.dylib"
	case "windows":
		if goarch == "386" {
			return "db2cli.dll"
		}
		return "db2cli64.dll"
	default:
		return "libdb2.so"
	}
}

// libDir returns the directory of home that holds the CLI library.
func libDir(home string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "bin")
	}
	return filepath.Join(home, "lib")
}

// LibPath returns the absolute library path inside a CLI driver home.
func LibPath(home string) string {
	return filepath.Join(libDir(home), LibName())
}

// Valid reports whether home contains the CLI library.
func Valid(home string) bool {
	if home == "" {
		return false
	}
	info, err := os.Stat(LibPath(home))
	return err == nil && !info.IsDir()
}

// ConfiguredHome returns the IBM_DB_HOME value, honouring the deprecated DB2HOME alias.
func ConfiguredHome() string {
	if v := os.Getenv(EnvHome); v != "" {
		return v
	}
	if v := os.Getenv(EnvLegacyHome); v != "" {
		Warn("%s is deprecated, use %s instead", EnvLegacyHome, EnvHome)
		return v
	}
	return ""
}

// AutoDownloadEnabled reports whether automatic provisioning is allowed.
func AutoDownloadEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvAutoDownload))) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// Resolution is the outcome of a library lookup.
type Resolution struct {
	Home     string   // CLI driver home that holds the library
	Lib      string   // absolute library path; empty when nothing was found
	Searched []string // every location that was checked
	// HomeInvalid is set when IBM_DB_HOME is set but does not hold the library.
	HomeInvalid bool
}

// Resolve searches IBM_DB_HOME, then the default directory. It does not download.
func Resolve() Resolution {
	var r Resolution

	if h := ConfiguredHome(); h != "" {
		p := LibPath(h)
		r.Searched = append(r.Searched, p)
		if Valid(h) {
			r.Home, r.Lib = h, p
			return r
		}
		r.HomeInvalid = true
	}

	if d, err := DefaultDir(); err == nil {
		p := LibPath(d)
		r.Searched = append(r.Searched, p)
		if Valid(d) {
			r.Home, r.Lib = d, p
			return r
		}
	}
	return r
}

// NotFoundError describes a failed lookup with remediation steps.
func NotFoundError(r Resolution, cause error) error {
	var b strings.Builder
	b.WriteString("Db2 CLI driver (" + LibName() + ") not found.\nSearched:\n")
	for _, p := range r.Searched {
		b.WriteString("  - " + p + "\n")
	}
	b.WriteString("  - the system library search path\n")
	if r.HomeInvalid {
		b.WriteString(EnvHome + " is set but does not contain the library; fix or unset it.\n")
	}
	if cause != nil {
		b.WriteString("Automatic download failed: " + cause.Error() + "\n")
	}
	b.WriteString("To fix, run: go run github.com/ibmdb/go_ibm_db/installer@latest\n")
	b.WriteString("or set " + EnvHome + " to an existing CLI driver directory.")
	return fmt.Errorf("%s", b.String())
}
