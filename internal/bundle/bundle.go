// Package bundle packages a challenge directory into a tarball, hashes it
// for content-addressable identity, and extracts it atomically into a
// worker's cache directory.
//
// Tar entries are emitted in sorted order and with normalized mtimes so the
// resulting bytes (and hash) are deterministic for a given input tree.
package bundle

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pack walks dir and returns a deterministic tarball plus the hex-encoded
// SHA-256 of its bytes. Symlinks are followed; device files are skipped.
func Pack(dir string) ([]byte, string, error) {
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, "", fmt.Errorf("stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := collect(dir)
	if err != nil {
		return nil, "", err
	}
	sort.Strings(entries)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, rel := range entries {
		abs := filepath.Join(dir, rel)
		fi, err := os.Stat(abs)
		if err != nil {
			return nil, "", fmt.Errorf("stat %s: %w", abs, err)
		}
		if !fi.Mode().IsRegular() && !fi.IsDir() {
			continue
		}
		hdr := &tar.Header{
			Name:    filepath.ToSlash(rel),
			Mode:    int64(fi.Mode().Perm()),
			ModTime: zeroTime,
		}
		if fi.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Name += "/"
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = fi.Size()
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, "", fmt.Errorf("write header %s: %w", rel, err)
		}
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Open(abs)
			if err != nil {
				return nil, "", fmt.Errorf("open %s: %w", abs, err)
			}
			if _, err := io.Copy(tw, f); err != nil {
				_ = f.Close()
				return nil, "", fmt.Errorf("copy %s: %w", rel, err)
			}
			_ = f.Close()
		}
	}
	if err := tw.Close(); err != nil {
		return nil, "", fmt.Errorf("close tar: %w", err)
	}

	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), nil
}

// Hash returns the hex-encoded SHA-256 of the given bundle bytes.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Extract writes the tarball into dest atomically: it extracts under a
// sibling tmp directory and renames into place. A nil error means dest
// holds the full extracted tree; an existing dest is left untouched.
func Extract(data []byte, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", dest, err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	tmp := dest + ".tmp." + suffix
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cleanup()
			return fmt.Errorf("read tar: %w", err)
		}
		if err := writeEntry(tmp, hdr, tr); err != nil {
			cleanup()
			return err
		}
	}

	if err := os.Rename(tmp, dest); err != nil {
		cleanup()
		// If another worker won the extraction race, dest is already populated.
		if _, statErr := os.Stat(dest); statErr == nil {
			return nil
		}
		return fmt.Errorf("rename %s → %s: %w", tmp, dest, err)
	}
	return nil
}

func writeEntry(root string, hdr *tar.Header, body io.Reader) error {
	clean := filepath.Clean(hdr.Name)
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("unsafe tar path: %s", hdr.Name)
	}
	target := filepath.Join(root, clean)

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, fs.FileMode(hdr.Mode)&fs.ModePerm)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode)&fs.ModePerm)
		if err != nil {
			return fmt.Errorf("create %s: %w", target, err)
		}
		if _, err := io.Copy(f, body); err != nil {
			_ = f.Close()
			return fmt.Errorf("write %s: %w", target, err)
		}
		return f.Close()
	default:
		return nil
	}
}

func collect(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out, err
}

func randomSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
