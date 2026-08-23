package rotatelog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Writer struct {
	mu               sync.Mutex
	dir, prefix, day string
	keep             int
	file             *os.File
}

func New(dir, prefix string, keepDays int) (*Writer, error) {
	w := &Writer{dir: dir, prefix: prefix, keep: keepDays}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	if err := w.rotate(time.Now()); err != nil {
		return nil, err
	}
	w.cleanup(time.Now())
	return w, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Now().Format("2006-01-02") != w.day {
		if err := w.rotate(time.Now()); err != nil {
			return 0, err
		}
		w.cleanup(time.Now())
	}
	return w.file.Write(p)
}

func (w *Writer) rotate(now time.Time) error {
	if w.file != nil {
		_ = w.file.Close()
	}
	w.day = now.Format("2006-01-02")
	f, err := os.OpenFile(filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, w.day)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err == nil {
		w.file = f
	}
	return err
}

func (w *Writer) cleanup(now time.Time) {
	files, _ := filepath.Glob(filepath.Join(w.dir, w.prefix+"-*.log"))
	cutoff := now.AddDate(0, 0, -w.keep)
	for _, name := range files {
		info, err := os.Stat(name)
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(name)
		}
	}
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
