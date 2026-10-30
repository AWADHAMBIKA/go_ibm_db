package clidriver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	primaryURL  = "https://public.dhe.ibm.com/ibmdl/export/pub/software/data/db2/drivers/odbc_cli/"
	fallbackURL = "https://github.com/ibmdb/db2drivers/raw/main/clidriver/"
)

// ArchiveName returns the CLI driver archive for a Linux or Windows architecture.
func ArchiveName(goos, goarch string) (string, error) {
	if goos == "windows" {
		switch goarch {
		case "amd64":
			return "ntx64_odbc_cli.zip", nil
		case "386":
			return "nt32_odbc_cli.zip", nil
		}
		return "", fmt.Errorf("no CLI driver archive is published for %s/%s", goos, goarch)
	}
	if goos != "linux" {
		return "", fmt.Errorf("automatic CLI driver download is not implemented for %s/%s", goos, goarch)
	}
	switch goarch {
	case "amd64":
		return "linuxx64_odbc_cli.tar.gz", nil
	case "ppc64le":
		return "ppc64le_odbc_cli.tar.gz", nil
	case "s390x":
		return "s390x64_odbc_cli.tar.gz", nil
	}
	return "", fmt.Errorf("no CLI driver archive is published for %s/%s", goos, goarch)
}

// downloadSource describes where to fetch the archive and how to verify it.
type downloadSource struct {
	urls   []string
	sha256 string
}

func resolveSource(archive string) (downloadSource, error) {
	var src downloadSource
	custom := os.Getenv(EnvDownloadURL)
	if custom == "" {
		if legacy := os.Getenv(EnvLegacyURL); legacy != "" {
			Warn("%s is deprecated, use %s instead", EnvLegacyURL, EnvDownloadURL)
			custom = legacy
		}
	}
	version := os.Getenv(EnvDownloadVersion)
	override := strings.ToLower(strings.TrimSpace(os.Getenv(EnvDownloadSHA256)))

	switch {
	case custom != "":
		src.urls = []string{custom}
	case version != "":
		src.urls = []string{primaryURL + version + "/" + archive, fallbackURL + version + "/" + archive}
	default:
		src.urls = []string{primaryURL + archive, fallbackURL + archive}
	}
	for _, u := range src.urls {
		if err := requireHTTPS(u); err != nil {
			return src, err
		}
	}

	// The checksum is optional: it is verified only when IBM_DB_DOWNLOAD_SHA256 is set.
	src.sha256 = override
	return src, nil
}

func requireHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid download URL %q: %w", raw, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("download URL %q must use https", raw)
	}
	return nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return requireHTTPS(req.URL.String())
		},
	}
}

// Download fetches rawURL into a new file at dst and verifies its SHA-256 when
// wantSHA256 is non-empty.
// On any failure dst is removed.
func Download(ctx context.Context, client *http.Client, rawURL, wantSHA256, dst string) (err error) {
	if err := requireHTTPS(rawURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", rawURL, resp.StatusCode)
	}

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(dst)
		}
	}()

	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return fmt.Errorf("saving %s: %w", rawURL, err)
	}
	if wantSHA256 == "" {
		return nil
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != strings.ToLower(wantSHA256) {
		return fmt.Errorf("SHA-256 mismatch for %s: expected %s, got %s", rawURL, wantSHA256, got)
	}
	return nil
}

// Install downloads the CLI driver for the current platform into dest.
func Install(ctx context.Context, dest string) error {
	archive, err := ArchiveName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	src, err := resolveSource(archive)
	if err != nil {
		return err
	}
	return installFrom(ctx, newHTTPClient(), src, dest)
}

func installFrom(ctx context.Context, client *http.Client, src downloadSource, dest string) error {
	dest = filepath.Clean(dest)
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(parent, ".clidriver-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	archivePath := filepath.Join(work, "archive")
	var lastErr error
	for _, u := range src.urls {
		Warn("downloading %s to %s", u, dest)
		if lastErr = Download(ctx, client, u, src.sha256, archivePath); lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return lastErr
	}

	extracted := filepath.Join(work, "x")
	if err := extractArchive(archivePath, extracted); err != nil {
		return err
	}

	// The published archives contain a single top-level "clidriver" directory.
	root := filepath.Join(extracted, "clidriver")
	if !Valid(root) {
		return fmt.Errorf("archive does not contain %s", filepath.Join("clidriver", "lib", LibName()))
	}

	if Valid(dest) {
		return nil // another process finished first
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.Rename(root, dest); err != nil {
		if Valid(dest) {
			return nil
		}
		return err
	}
	return nil
}

// Ensure resolves the CLI driver, downloading it into the default directory when
// it is missing and automatic downloads are enabled.
func Ensure(ctx context.Context) (Resolution, error) {
	r := Resolve()
	if r.Lib != "" {
		return r, nil
	}
	if !AutoDownloadEnabled() {
		return r, NotFoundError(r, fmt.Errorf("%s=0 disables automatic download", EnvAutoDownload))
	}
	dest, err := DefaultDir()
	if err != nil {
		return r, NotFoundError(r, err)
	}
	if err := Install(ctx, dest); err != nil {
		return r, NotFoundError(r, err)
	}
	return Resolve(), nil
}
