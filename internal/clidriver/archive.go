package clidriver

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxEntrySize bounds a single extracted file to defend against decompression bombs.
const maxEntrySize = 2 << 30

// safeJoin resolves name under root and rejects anything that escapes it.
func safeJoin(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return target, nil
}

// extractArchive extracts a downloaded .zip or .tar.gz file, chosen by its content.
func extractArchive(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("invalid archive: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if string(magic) == "PK\x03\x04" {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		return ExtractZip(f, info.Size(), dest)
	}
	return ExtractTarGz(f, dest)
}

// ExtractZip extracts a .zip archive into dest, rejecting unsafe entries.
func ExtractZip(r io.ReaderAt, size int64, dest string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}
	dest = filepath.Clean(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, zf := range zr.File {
		target, err := safeJoin(dest, strings.TrimSuffix(zf.Name, "/"))
		if err != nil {
			return err
		}
		mode := zf.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			if zf.UncompressedSize64 > maxEntrySize {
				return fmt.Errorf("archive entry %q is too large", zf.Name)
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			err = writeFile(target, rc, mode.Perm())
			rc.Close()
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %q", zf.Name)
		}
	}
	return nil
}

// ExtractTarGz extracts a .tar.gz stream into dest, rejecting unsafe entries.
func ExtractTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gz.Close()

	dest = filepath.Clean(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading archive: %w", err)
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if hdr.Size > maxEntrySize {
				return fmt.Errorf("archive entry %q is too large", hdr.Name)
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := checkLink(dest, target, hdr.Linkname); err != nil {
				return fmt.Errorf("unsafe symlink %q: %w", hdr.Name, err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			src, err := safeJoin(dest, hdr.Linkname)
			if err != nil {
				return fmt.Errorf("unsafe hard link %q: %w", hdr.Name, err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(src, target); err != nil {
				return err
			}
		default:
			// Devices, FIFOs and other special entries are never needed.
			return fmt.Errorf("unsupported archive entry type for %q", hdr.Name)
		}
	}
}

// checkLink ensures a symlink at linkPath pointing to linkname stays inside root.
func checkLink(root, linkPath, linkname string) error {
	if filepath.IsAbs(linkname) {
		return fmt.Errorf("absolute target %q", linkname)
	}
	resolved := filepath.Join(filepath.Dir(linkPath), linkname)
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target %q escapes the destination", linkname)
	}
	return nil
}

func writeFile(path string, src io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode|0o200)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(src, maxEntrySize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxEntrySize {
		err = fmt.Errorf("archive entry %q is too large", path)
	}
	return err
}
