package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/ibmdb/go_ibm_db/internal/clidriver"
	trc "github.com/ibmdb/go_ibm_db/log2"
)

// loadWithAlteredSearchPath makes Windows resolve the DLL's own dependencies
// from the directory of the DLL, so PATH does not need to contain clidriver\bin.
const loadWithAlteredSearchPath = 0x8

var (
	loadMu    sync.Mutex
	loaded    bool
	loadErr   error
	dllHandle syscall.Handle

	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLoadLibraryW = kernel32.NewProc("LoadLibraryExW")
)

func init() {
	LoadError = func() error {
		loadMu.Lock()
		defer loadMu.Unlock()
		return loadErr
	}
}

// lazyLib and lazyProc resolve Db2 CLI entry points from the DLL that
// ensureLoaded loaded by absolute path.
type lazyLib struct{}

type lazyProc struct {
	name string
	addr uintptr
}

var mododbc32 = &lazyLib{}

func (*lazyLib) NewProc(name string) *lazyProc { return &lazyProc{name: name} }

func (p *lazyProc) Addr() uintptr {
	if a := atomic.LoadUintptr(&p.addr); a != 0 {
		return a
	}
	loadMu.Lock()
	h := dllHandle
	loadMu.Unlock()
	if h == 0 {
		panic(fmt.Sprintf("api: Db2 CLI is not loaded (%s): %v", p.name, LoadError()))
	}
	a, err := syscall.GetProcAddress(h, p.name)
	if err != nil {
		panic(fmt.Sprintf("api: Db2 CLI function %s not found: %v", p.name, err))
	}
	atomic.StoreUintptr(&p.addr, a)
	return a
}

// ensureLoaded locates the CLI DLL and loads it by absolute path.
// Success is final; a failure is retried on the next call.
func ensureLoaded() bool {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded {
		return true
	}
	h, err := loadLibrary()
	if loadErr = err; err != nil {
		trc.Trace1(fmt.Sprintf("api: loading Db2 CLI failed: %v", err))
		return false
	}
	dllHandle, loaded = h, true
	return true
}

func loadLibrary() (syscall.Handle, error) {
	r := clidriver.Resolve()
	if r.Lib == "" {
		if p := findOnPath(clidriver.LibName()); p != "" {
			return loadAbs(p)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var err error
		if r, err = clidriver.Ensure(ctx); err != nil {
			return 0, err
		}
	}
	h, err := loadAbs(r.Lib)
	if err != nil {
		return 0, fmt.Errorf("loading %s failed: %w\nSet %s to a valid Db2 CLI driver directory or remove %s", r.Lib, err, clidriver.EnvHome, r.Home)
	}
	return h, nil
}

// findOnPath keeps existing installations that rely on PATH working, but still
// loads the library by absolute path.
func findOnPath(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func loadAbs(path string) (syscall.Handle, error) {
	if !filepath.IsAbs(path) {
		return 0, fmt.Errorf("refusing to load %q: path is not absolute", path)
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, _, e := procLoadLibraryW.Call(uintptr(unsafe.Pointer(p)), 0, loadWithAlteredSearchPath)
	if h == 0 {
		return 0, e
	}
	return syscall.Handle(h), nil
}
