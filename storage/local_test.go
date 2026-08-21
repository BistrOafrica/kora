package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestLocalBackendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	b, err := New(Config{Backend: "local", LocalPath: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	key := "sites/acme/files/2026/08/hello.txt"
	content := []byte("hello world")
	meta, err := b.Put(context.Background(), key, bytes.NewReader(content), int64(len(content)), FileMeta{Filename: "hello.txt", MIMEType: "text/plain"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if meta.Size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", meta.Size, len(content))
	}
	if meta.Checksum == "" {
		t.Fatalf("expected checksum to be set")
	}

	head, err := b.Head(context.Background(), key)
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head.Size != int64(len(content)) {
		t.Fatalf("head size = %d, want %d", head.Size, len(content))
	}
	if head.MIMEType != "text/plain" {
		t.Fatalf("mime = %q, want text/plain", head.MIMEType)
	}

	rc, err := b.Open(context.Background(), key, 0, -1)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, content) {
		t.Fatalf("content = %q, want %q", got, content)
	}

	// Range read: bytes [6, 10] = "world".
	rc2, err := b.Open(context.Background(), key, 6, 5)
	if err != nil {
		t.Fatalf("Open range: %v", err)
	}
	got2, _ := io.ReadAll(rc2)
	rc2.Close()
	if string(got2) != "world" {
		t.Fatalf("range = %q, want world", got2)
	}

	if err := b.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := b.Head(context.Background(), key); err != ErrNotFound {
		t.Fatalf("Head after delete = %v, want ErrNotFound", err)
	}
}

func TestLocalBackendPersistsMetadata(t *testing.T) {
	dir := t.TempDir()
	b, err := New(Config{Backend: "local", LocalPath: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	key := "sites/acme/files/2026/08/recording.mp3"
	content := []byte("fake-audio")
	uploadedAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	meta, err := b.Put(context.Background(), key, bytes.NewReader(content), int64(len(content)), FileMeta{
		Filename:   "voice-note.mp3",
		MIMEType:   "audio/mpeg",
		UploadedBy: "alice",
		UploadedAt: uploadedAt,
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	head, err := b.Head(context.Background(), key)
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head.Filename != "voice-note.mp3" {
		t.Fatalf("filename = %q, want voice-note.mp3", head.Filename)
	}
	if head.MIMEType != "audio/mpeg" {
		t.Fatalf("mime = %q, want audio/mpeg", head.MIMEType)
	}
	if head.Checksum != meta.Checksum || head.Checksum == "" {
		t.Fatalf("checksum = %q, want %q", head.Checksum, meta.Checksum)
	}
	if head.UploadedBy != "alice" {
		t.Fatalf("uploaded_by = %q, want alice", head.UploadedBy)
	}
}
