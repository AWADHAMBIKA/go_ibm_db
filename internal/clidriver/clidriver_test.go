package clidriver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type entry struct {
	name, link string
	typ        byte
	body       string
}

func makeTarGz(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typ, Mode: 0o644, Linkname: e.link}
		if typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if typ == tar.TypeDir {
			hdr.Mode = 0o755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestExtractRejectsUnsafePaths(t *testing.T) {
	cases := map[string][]entry{
		"parent traversal":   {{name: "../evil.txt", body: "x"}},
		"nested traversal":   {{name: "clidriver/../../evil.txt", body: "x"}},
		"absolute path":      {{name: "/tmp/evil.txt", body: "x"}},
		"symlink escape":     {{name: "clidriver/l", typ: tar.TypeSymlink, link: "../../outside"}},
		"absolute symlink":   {{name: "clidriver/l", typ: tar.TypeSymlink, link: "/etc/passwd"}},
		"hard link escape":   {{name: "clidriver/h", typ: tar.TypeLink, link: "../outside"}},
		"special file entry": {{name: "clidriver/fifo", typ: tar.TypeFifo}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dest := filepath.Join(root, "dest")
			err := ExtractTarGz(bytes.NewReader(makeTarGz(t, entries)), dest)
			if err == nil {
				t.Fatal("expected an error")
			}
			if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); statErr == nil {
				t.Fatal("file was written outside the destination")
			}
		})
	}
}

func TestExtractValidArchive(t *testing.T) {
	dest := t.TempDir()
	data := makeTarGz(t, []entry{
		{name: "clidriver/", typ: tar.TypeDir},
		{name: "clidriver/lib/libdb2.so.1", body: "lib"},
		{name: "clidriver/lib/libdb2.so", typ: tar.TypeSymlink, link: "libdb2.so.1"},
	})
	if err := ExtractTarGz(bytes.NewReader(data), dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "clidriver", "lib", "libdb2.so"))
	if err != nil || string(got) != "lib" {
		t.Fatalf("symlink not usable: %q, %v", got, err)
	}
}

func tlsServer(t *testing.T, body []byte) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, srv.Client()
}

func TestDownloadChecksum(t *testing.T) {
	body := []byte("payload")
	srv, client := tlsServer(t, body)
	dst := filepath.Join(t.TempDir(), "a.tgz")

	if err := Download(context.Background(), client, srv.URL, sum(body), dst); err != nil {
		t.Fatalf("valid checksum rejected: %v", err)
	}

	bad := filepath.Join(t.TempDir(), "b.tgz")
	err := Download(context.Background(), client, srv.URL, strings.Repeat("0", 64), bad)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected checksum error, got %v", err)
	}
	if _, statErr := os.Stat(bad); statErr == nil {
		t.Fatal("file with bad checksum was left on disk")
	}
}

func TestDownloadRequiresHTTPS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	err := Download(context.Background(), http.DefaultClient, srv.URL, "x", filepath.Join(t.TempDir(), "f"))
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https error, got %v", err)
	}
}

func TestInstallFrom(t *testing.T) {
	archive := makeTarGz(t, []entry{
		{name: "clidriver/", typ: tar.TypeDir},
		{name: "clidriver/lib/" + LibName(), body: "lib"},
		{name: "clidriver/bin/" + LibName(), body: "lib"},
	})
	srv, client := tlsServer(t, archive)
	dest := filepath.Join(t.TempDir(), ".ibm_db", "clidriver")

	err := installFrom(context.Background(), client, downloadSource{urls: []string{srv.URL}, sha256: sum(archive)}, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !Valid(dest) {
		t.Fatal("installed directory is not a valid CLI driver")
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".clidriver-install-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}

	err = installFrom(context.Background(), client, downloadSource{urls: []string{srv.URL}, sha256: strings.Repeat("1", 64)}, filepath.Join(t.TempDir(), "x"))
	if err == nil {
		t.Fatal("expected checksum failure")
	}
}

func TestResolveHomeTakesPrecedence(t *testing.T) {
	home := t.TempDir()
	libPath := LibPath(home)
	if err := os.MkdirAll(filepath.Dir(libPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvHome, home)
	t.Setenv("HOME", t.TempDir())

	r := Resolve()
	if r.Lib != libPath || r.Home != home {
		t.Fatalf("unexpected resolution: %+v", r)
	}
}

func TestResolveDefaultDirAndNotFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv(EnvHome, "")
	t.Setenv(EnvLegacyHome, "")

	r := Resolve()
	if r.Lib != "" {
		t.Fatalf("expected nothing found, got %+v", r)
	}
	err := NotFoundError(r, nil)
	if !strings.Contains(err.Error(), filepath.Join(fakeHome, ".ibm_db", "clidriver")) {
		t.Fatalf("error should list searched paths: %v", err)
	}

	def := filepath.Join(fakeHome, ".ibm_db", "clidriver")
	os.MkdirAll(filepath.Dir(LibPath(def)), 0o755)
	os.WriteFile(LibPath(def), nil, 0o644)
	if r := Resolve(); r.Home != def {
		t.Fatalf("default dir not used: %+v", r)
	}
}

func TestEnsureHonoursAutoDownloadOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(EnvHome, "")
	t.Setenv(EnvLegacyHome, "")
	t.Setenv(EnvAutoDownload, "0")

	_, err := Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Searched") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestResolveSourceRules(t *testing.T) {
	t.Setenv(EnvDownloadURL, "http://example.com/a.tar.gz")
	t.Setenv(EnvDownloadSHA256, strings.Repeat("a", 64))
	if _, err := resolveSource("linuxx64_odbc_cli.tar.gz"); err == nil {
		t.Fatal("http URL must be rejected")
	}

	t.Setenv(EnvDownloadURL, "https://example.com/a.tar.gz")
	t.Setenv(EnvDownloadSHA256, "")
	if _, err := resolveSource("linuxx64_odbc_cli.tar.gz"); err != nil {
		t.Fatalf("checksum is optional: %v", err)
	}

	t.Setenv(EnvDownloadSHA256, strings.Repeat("a", 64))
	if _, err := resolveSource("linuxx64_odbc_cli.tar.gz"); err != nil {
		t.Fatalf("custom URL with checksum should be accepted: %v", err)
	}
}

func TestArchiveName(t *testing.T) {
	for arch, want := range map[string]string{
		"amd64":   "linuxx64_odbc_cli.tar.gz",
		"ppc64le": "ppc64le_odbc_cli.tar.gz",
		"s390x":   "s390x64_odbc_cli.tar.gz",
	} {
		got, err := ArchiveName("linux", arch)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v", arch, got, err)
		}
	}
	if _, err := ArchiveName("linux", "riscv64"); err == nil {
		t.Error("unsupported arch should error")
	}
}
