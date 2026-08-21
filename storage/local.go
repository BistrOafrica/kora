package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

type localBackend struct {
	root string
}

func newLocalBackend(cfg Config) *localBackend {
	root := cfg.LocalPath
	if root == "" {
		root = "."
	}
	return &localBackend{root: root}
}

func (b *localBackend) fullPath(key string) string {
	return filepath.Join(b.root, filepath.FromSlash(key))
}

func (b *localBackend) Put(_ context.Context, key string, r io.Reader, _ int64, meta FileMeta) (*FileMeta, error) {
	full := b.fullPath(key)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return nil, err
	}
	out, err := os.Create(full)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hasher), r)
	if err != nil {
		return nil, err
	}

	meta.Key = key
	meta.Size = written
	meta.Checksum = "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if meta.Filename == "" {
		meta.Filename = baseName(key)
	}
	if meta.MIMEType == "" {
		meta.MIMEType = MimeByExt(extOf(key))
	}
	if meta.UploadedAt.IsZero() {
		meta.UploadedAt = time.Now().UTC()
	}
	meta.ModifiedAt = meta.UploadedAt

	// Persist exact metadata alongside the blob for later retrieval.
	if data, err := json.Marshal(meta); err == nil {
		_ = os.WriteFile(full+".meta.json", data, 0644)
	}
	return &meta, nil
}

func (b *localBackend) Head(_ context.Context, key string) (*FileMeta, error) {
	full := b.fullPath(key)
	st, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if st.IsDir() {
		return nil, ErrNotFound
	}
	meta := &FileMeta{
		Key:        key,
		Filename:   baseName(key),
		MIMEType:   MimeByExt(extOf(key)),
		Size:       st.Size(),
		ModifiedAt: st.ModTime(),
	}
	if data, err := os.ReadFile(full + ".meta.json"); err == nil {
		var side FileMeta
		if json.Unmarshal(data, &side) == nil {
			if side.Filename != "" {
				meta.Filename = side.Filename
			}
			if side.MIMEType != "" {
				meta.MIMEType = side.MIMEType
			}
			if side.Checksum != "" {
				meta.Checksum = side.Checksum
			}
			if !side.UploadedAt.IsZero() {
				meta.UploadedAt = side.UploadedAt
			}
			meta.UploadedBy = side.UploadedBy
		}
	}
	return meta, nil
}

func (b *localBackend) Open(_ context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	f, err := os.Open(b.fullPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
	}
	if length < 0 {
		return f, nil
	}
	return &readCloser{Reader: io.LimitReader(f, length), Closer: f}, nil
}

func (b *localBackend) Delete(_ context.Context, key string) error {
	full := b.fullPath(key)
	err := os.Remove(full)
	if os.IsNotExist(err) {
		err = nil
	}
	_ = os.Remove(full + ".meta.json")
	return err
}

func (b *localBackend) URL(_ context.Context, _ string) (string, error) {
	return "", nil
}

// readCloser adapts an io.Reader + io.Closer pair into an io.ReadCloser.
type readCloser struct {
	io.Reader
	io.Closer
}
