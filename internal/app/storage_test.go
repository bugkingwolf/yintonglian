package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A genuinely non-disk adapter: integration tests fail if a handler or backup
// bypasses FileStorage and tries to read DATA_DIR/uploads directly.
type memoryStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	failPut bool
}

func newMemoryStorage() *memoryStorage { return &memoryStorage{objects: map[string][]byte{}} }
func (m *memoryStorage) Put(ctx context.Context, key string, r io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failPut {
		return fs.ErrPermission
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	m.objects[key] = b
	return nil
}
func (m *memoryStorage) Open(ctx context.Context, key string) (StoredObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	b, ok := m.objects[key]
	if !ok {
		return StoredObject{}, fs.ErrNotExist
	}
	return StoredObject{ReadSeekCloser: memoryObject{bytes.NewReader(b)}, ObjectInfo: ObjectInfo{Size: int64(len(b)), Modified: time.Unix(1, 0)}}, nil
}
func (m *memoryStorage) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	o, e := m.Open(ctx, key)
	if e != nil {
		return ObjectInfo{}, e
	}
	o.Close()
	return o.ObjectInfo, nil
}
func (m *memoryStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	delete(m.objects, key)
	return nil
}

type memoryObject struct{ *bytes.Reader }

func (memoryObject) Close() error { return nil }

func TestStorageAdapterHTTPAndBackup(t *testing.T) {
	files := newMemoryStorage()
	h := newStorageHarness(t, files)
	h.setup(t)
	up := uploadImage(t, h)
	url := up["url"].(string)
	key := strings.TrimPrefix(url, "/media/")
	if _, err := os.Stat(filepath.Join(h.dataDir, "uploads")); !os.IsNotExist(err) {
		t.Fatal("injected backend should not require local uploads directory")
	}
	status, _, _ := h.call(t, "GET", url, nil, false, nil)
	expect(t, status, 404)
	status, partial, res := h.call(t, "GET", url, nil, true, map[string]string{"Range": "bytes=0-7"})
	expect(t, status, 206)
	if !bytes.Equal(partial["bytes"].([]byte), []byte("\x89PNG\r\n\x1a\n")) || res.Header.Get("Content-Range") == "" {
		t.Fatal("range response changed")
	}
	status, _, res = h.call(t, "HEAD", url, nil, true, nil)
	expect(t, status, 200)
	if res.Header.Get("Content-Length") == "" {
		t.Fatal("HEAD metadata missing")
	}
	c := contentCase(t, h, "life-forward")
	original := c.Cover
	c.Cover = url
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+c.ID, c, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "GET", url, nil, false, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+up["id"].(string), nil, true, nil)
	expect(t, status, 409)
	dest := filepath.Join(t.TempDir(), "backup")
	if err := h.app.Backup(dest); err != nil {
		t.Fatal(err)
	}
	restored, err := New(Config{Root: h.root, DataDir: dest})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	object, err := restored.Files.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	b, err := io.ReadAll(object)
	if err != nil || !bytes.Equal(b, files.objects[key]) {
		t.Fatal("backup did not preserve bytes across adapters")
	}
	c.Cover = original
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+c.ID, c, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+up["id"].(string), nil, true, nil)
	expect(t, status, 200)
	if len(files.objects) != 0 {
		t.Fatal("delete bypassed storage adapter")
	}
}

func TestStorageFailuresDoNotLeaveUploadRecordsOrObjects(t *testing.T) {
	files := newMemoryStorage()
	h := newStorageHarness(t, files)
	h.setup(t)
	files.failPut = true
	status, _, _ := h.call(t, "POST", "/api/admin/uploads", []byte("\x89PNG\r\n\x1a\n"), true, map[string]string{"X-File-Name": "test.png"})
	expect(t, status, 500)
	files.failPut = false
	if err := h.app.Store.DB.Exec("CREATE TRIGGER reject_upload BEFORE INSERT ON uploads BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	status, _, _ = h.call(t, "POST", "/api/admin/uploads", []byte("\x89PNG\r\n\x1a\n"), true, map[string]string{"X-File-Name": "test.png"})
	expect(t, status, 500)
	items, err := h.app.Store.Uploads()
	if err != nil || len(items) != 0 || len(files.objects) != 0 {
		t.Fatal("failed upload left records or objects")
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestLocalStorageAtomicityAndConfinement(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "abc123.png"
	if err = s.Put(ctx, key, strings.NewReader("original")); err != nil {
		t.Fatal(err)
	}
	err = s.Put(ctx, key, io.MultiReader(strings.NewReader("partial"), brokenReader{}))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil || string(b) != "original" {
		t.Fatal("failed write replaced original")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary object leaked")
	}
	for _, badKey := range []string{"../abc123.png", "/abc123.png", "abc123.tmp"} {
		if _, err = s.Stat(ctx, badKey); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("accepted invalid key %q", badKey)
		}
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err = os.WriteFile(target, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(dir, "def456.png")); err == nil {
		if _, err = s.Open(ctx, "def456.png"); err == nil {
			t.Fatal("opened symlink outside storage root")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = s.Put(canceled, key, strings.NewReader("changed")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(ctx, key); err != nil {
		t.Fatal("delete is not idempotent")
	}
}

type signedStorage struct {
	*memoryStorage
	signCalls int
}

func (s *signedStorage) SignedURL(ctx context.Context, key, method string, ttl time.Duration) (string, error) {
	s.signCalls++
	if method != "GET" && method != "HEAD" {
		return "", errors.New("unexpected method")
	}
	if ttl > 5*time.Minute {
		return "", errors.New("unexpected lifetime")
	}
	return "https://media.example.invalid/" + key + "?signature=test-only", nil
}
func TestSignedMediaDeliveryRunsAfterAuthorization(t *testing.T) {
	files := &signedStorage{memoryStorage: newMemoryStorage()}
	h := newStorageHarness(t, files)
	h.setup(t)
	// Never make an outbound request: inspect the local redirect response only.
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	up := uploadImage(t, h)
	path := up["url"].(string)
	status, _, _ := h.call(t, "GET", path, nil, false, nil)
	expect(t, status, 404)
	if files.signCalls != 0 {
		t.Fatal("private upload received a signed URL without auth")
	}
	status, _, res := h.call(t, "GET", path, nil, true, nil)
	expect(t, status, 307)
	if !strings.HasPrefix(res.Header.Get("Location"), "https://media.example.invalid/") || res.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("signed delivery response invalid")
	}
	status, _, _ = h.call(t, "HEAD", path, nil, true, nil)
	expect(t, status, 307)
	if files.signCalls != 2 {
		t.Fatal("authorized delivery did not use signing adapter")
	}
}
