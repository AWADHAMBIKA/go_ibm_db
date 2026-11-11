package clidriver

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "required privilege") {
			t.Skip("creating symlinks requires privileges on this Windows machine")
		}
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

	err := installFrom(context.Background(), client, downloadSource{urls: []string{srv.URL}, sha256: sum(archive)}, dest, false)
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

	err = installFrom(context.Background(), client, downloadSource{urls: []string{srv.URL}, sha256: strings.Repeat("1", 64)}, filepath.Join(t.TempDir(), "x"), false)
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
	t.Setenv("USERPROFILE", t.TempDir())

	r := Resolve()
	if r.Lib != libPath || r.Home != home {
		t.Fatalf("unexpected resolution: %+v", r)
	}
}

func TestResolveDefaultDirAndNotFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)
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
	t.Setenv("USERPROFILE", t.TempDir())
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
	if _, err := resolveSourceFor("linux", "amd64", "linuxx64_odbc_cli.tar.gz"); err == nil {
		t.Fatal("http URL must be rejected")
	}

	t.Setenv(EnvDownloadURL, "https://example.com/a.tar.gz")
	t.Setenv(EnvDownloadSHA256, "")
	if _, err := resolveSourceFor("linux", "amd64", "linuxx64_odbc_cli.tar.gz"); err != nil {
		t.Fatalf("checksum is optional: %v", err)
	}

	t.Setenv(EnvDownloadSHA256, strings.Repeat("a", 64))
	if _, err := resolveSourceFor("linux", "amd64", "linuxx64_odbc_cli.tar.gz"); err != nil {
		t.Fatalf("custom URL with checksum should be accepted: %v", err)
	}
}

func TestForceDownloadFlag(t *testing.T) {
	t.Setenv(EnvForceDownload, "true")
	if !ForceDownload() {
		t.Fatal("DOWNLOAD_CLIDRIVER=true should force a download")
	}
	t.Setenv(EnvForceDownload, "0")
	if ForceDownload() {
		t.Fatal("DOWNLOAD_CLIDRIVER=0 should not force a download")
	}
}

func TestEnsureIgnoresForceDownload(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(EnvLegacyHome, "")
	t.Setenv(EnvAutoDownload, "")
	// Any download attempt would fail: plain HTTP is rejected.
	t.Setenv(EnvDownloadURL, "http://127.0.0.1:1/archive")
	t.Setenv(EnvForceDownload, "true")

	existing := t.TempDir()
	os.MkdirAll(filepath.Dir(LibPath(existing)), 0o755)
	if err := os.WriteFile(LibPath(existing), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvHome, existing)

	r, err := Ensure(context.Background())
	if err != nil || r.Home != existing {
		t.Fatalf("DOWNLOAD_CLIDRIVER must not affect the runtime loader: %+v, %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ibm_db")); err == nil {
		t.Fatal("nothing should be written to the default directory")
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

func TestArchiveNameWindows(t *testing.T) {
	for arch, want := range map[string]string{
		"amd64": "ntx64_odbc_cli.zip",
		"386":   "nt32_odbc_cli.zip",
	} {
		got, err := ArchiveName("windows", arch)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v", arch, got, err)
		}
	}
	if _, err := ArchiveName("windows", "arm64"); err == nil {
		t.Error("windows/arm64 should error")
	}
}

func TestArchiveNameDarwin(t *testing.T) {
	for arch, want := range map[string]string{
		"amd64": "macos64_odbc_cli.tar.gz",
		"arm64": "macarm64_odbc_cli.tar.gz",
	} {
		got, err := ArchiveName("darwin", arch)
		if err != nil || got != want {
			t.Errorf("darwin/%s: got %q, %v; want %q", arch, got, err, want)
		}
	}
	if _, err := ArchiveName("darwin", "386"); err == nil {
		t.Fatal("expected unsupported architecture error")
	}
}

func TestDarwinDownloadChecksums(t *testing.T) {
	t.Setenv(EnvDownloadURL, "")
	t.Setenv(EnvLegacyURL, "")
	t.Setenv(EnvDownloadVersion, "")
	t.Setenv(EnvDownloadSHA256, "")
	for arch, version := range map[string]string{"amd64": "v11.5.9", "arm64": "v12.1.4"} {
		archive, err := ArchiveName("darwin", arch)
		if err != nil {
			t.Fatal(err)
		}
		src, err := resolveSourceFor("darwin", arch, archive)
		if err != nil || len(src.sha256) != 64 || !strings.Contains(src.urls[0], "/"+version+"/") {
			t.Fatalf("darwin/%s: unexpected source %+v, %v", arch, src, err)
		}
	}
	t.Setenv(EnvDownloadVersion, "v12.1.2")
	if _, err := resolveSourceFor("darwin", "arm64", "archive"); err == nil {
		t.Fatal("expected checksum requirement for an unpinned version")
	}
	t.Setenv(EnvDownloadSHA256, strings.Repeat("a", 64))
	if _, err := resolveSourceFor("darwin", "arm64", "archive"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvDownloadSHA256, "")
	t.Setenv(EnvDownloadURL, "https://example.com/archive")
	if _, err := resolveSourceFor("darwin", "arm64", "archive"); err == nil {
		t.Fatal("expected checksum requirement for a custom URL")
	}
}

func TestLibNameWindows(t *testing.T) {
	if got := libNameFor("windows", "amd64"); got != "db2cli64.dll" {
		t.Errorf("amd64: %s", got)
	}
	if got := libNameFor("windows", "386"); got != "db2cli.dll" {
		t.Errorf("386: %s", got)
	}
}

func makeZip(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.body))
	}
	zw.Close()
	return buf.Bytes()
}

func TestExtractZipRejectsUnsafePaths(t *testing.T) {
	for name, e := range map[string]entry{
		"parent traversal": {name: "../evil.txt", body: "x"},
		"nested traversal": {name: "a/../../evil.txt", body: "x"},
		"absolute":         {name: "/evil.txt", body: "x"},
		"backslash root":   {name: `\evil.txt`, body: "x"},
	} {
		data := makeZip(t, []entry{e})
		dest := filepath.Join(t.TempDir(), "x")
		if err := ExtractZip(bytes.NewReader(data), int64(len(data)), dest); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestInstallFromZip(t *testing.T) {
	archive := makeZip(t, []entry{
		{name: "clidriver/lib/" + LibName(), body: "lib"},
		{name: "clidriver/bin/" + LibName(), body: "lib"},
	})
	srv, client := tlsServer(t, archive)
	dest := filepath.Join(t.TempDir(), ".ibm_db", "clidriver")
	if err := installFrom(context.Background(), client, downloadSource{urls: []string{srv.URL}, sha256: sum(archive)}, dest, false); err != nil {
		t.Fatal(err)
	}
	if !Valid(dest) {
		t.Fatal("installed directory is not a valid CLI driver")
	}
}

func TestInstallFromReplace(t *testing.T) {
	archive := makeTarGz(t, []entry{
		{name: "clidriver/", typ: tar.TypeDir},
		{name: "clidriver/lib/" + LibName(), body: "new"},
		{name: "clidriver/bin/" + LibName(), body: "new"},
	})
	srv, client := tlsServer(t, archive)
	dest := filepath.Join(t.TempDir(), ".ibm_db", "clidriver")
	os.MkdirAll(filepath.Dir(LibPath(dest)), 0o755)
	if err := os.WriteFile(LibPath(dest), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	bad := downloadSource{urls: []string{srv.URL}, sha256: strings.Repeat("1", 64)}
	if err := installFrom(context.Background(), client, bad, dest, true); err == nil {
		t.Fatal("expected checksum failure")
	}
	if got, _ := os.ReadFile(LibPath(dest)); string(got) != "old" {
		t.Fatalf("failed download must keep the existing install, got %q", got)
	}

	good := downloadSource{urls: []string{srv.URL}, sha256: sum(archive)}
	if err := installFrom(context.Background(), client, good, dest, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(LibPath(dest)); string(got) != "old" {
		t.Fatalf("non-replace install must keep a valid install, got %q", got)
	}
	if err := installFrom(context.Background(), client, good, dest, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(LibPath(dest)); string(got) != "new" {
		t.Fatalf("replace install should install the new copy, got %q", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".clidriver-install-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}
