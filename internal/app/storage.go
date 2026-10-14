package app

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ObjectInfo is independent of local paths and provider-specific URLs.
type ObjectInfo struct {
	Size     int64
	Modified time.Time
}

type StoredObject struct {
	io.ReadSeekCloser
	ObjectInfo
}

// FileStorage owns persistent upload bytes, not authentication or database records.
// Keys are immutable identifiers such as "<uuid>.jpg", never paths or public URLs.
// Open must support seeking so HTTP Range and HEAD work for videos. A remote adapter
// can implement range reads; it must not redirect private files to permanent URLs.
// Put publishes a complete object atomically. Delete is idempotent for absent keys.
type FileStorage interface {
	Put(context.Context, string, io.Reader) error
	Open(context.Context, string) (StoredObject, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
}

// SignedMediaStorage optionally delivers directly from object storage/CDN instead
// of proxying video bytes through the application server. Authorization always
// runs first. Implementations must sign the requested method with this lifetime.
type SignedMediaStorage interface {
	SignedURL(context.Context, string, string, time.Duration) (string, error)
}

// LocalFileStorage retains the existing DATA_DIR/uploads layout.
type LocalFileStorage struct{ directory string }

func NewLocalFileStorage(directory string) (*LocalFileStorage, error) {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	return &LocalFileStorage{directory: abs}, nil
}
func (s *LocalFileStorage) root(ctx context.Context, key string) (*os.Root, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !uploadPattern.MatchString(key) {
		return nil, fs.ErrInvalid
	}
	// Root confines operations to this directory even if a symlink is encountered.
	return os.OpenRoot(s.directory)
}
func (s *LocalFileStorage) Put(ctx context.Context, key string, source io.Reader) error {
	root, err := s.root(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	suffix, err := newID()
	if err != nil {
		return err
	}
	tmp := key + "." + suffix + ".tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	defer f.Close()
	if _, err = io.Copy(f, contextReader{ctx: ctx, reader: source}); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return root.Rename(tmp, key)
}
func (s *LocalFileStorage) Open(ctx context.Context, key string) (StoredObject, error) {
	root, err := s.root(ctx, key)
	if err != nil {
		return StoredObject{}, err
	}
	defer root.Close()
	f, err := root.Open(key)
	if err != nil {
		return StoredObject{}, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return StoredObject{}, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return StoredObject{}, fs.ErrNotExist
	}
	return StoredObject{ReadSeekCloser: f, ObjectInfo: ObjectInfo{Size: info.Size(), Modified: info.ModTime()}}, nil
}
func (s *LocalFileStorage) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	root, err := s.root(ctx, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer root.Close()
	info, err := root.Stat(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if !info.Mode().IsRegular() {
		return ObjectInfo{}, fs.ErrNotExist
	}
	return ObjectInfo{Size: info.Size(), Modified: info.ModTime()}, nil
}
func (s *LocalFileStorage) Delete(ctx context.Context, key string) error {
	root, err := s.root(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	err = root.Remove(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
