package rotatelog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriterRemovesExpiredLogs(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "tgproxy-2000-01-01.log")
	if err := os.WriteFile(old, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().AddDate(0, 0, -8)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	w, err := New(dir, "tgproxy", 7)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("expired log still exists: %v", err)
	}
	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
}
