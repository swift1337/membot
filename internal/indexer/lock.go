package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrLockHeld = errors.New("index lock already held")

type lockRecord struct {
	PID       int   `json:"pid"`
	StartedAt int64 `json:"started_at_unix"`
}

type Lock struct {
	path string
	file *os.File
}

func DefaultLockPath() string {
	return filepath.Join(storeDataDir(), "index.lock")
}

func DefaultLogDir() string {
	return filepath.Join(storeDataDir(), "logs")
}

func storeDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".membot"
	}
	return filepath.Join(home, ".membot")
}

func AcquireLock(path string) (*Lock, error) {
	if path == "" {
		path = DefaultLockPath()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	lock, err := tryAcquireLock(path)
	if err == nil {
		return lock, nil
	}
	if !errors.Is(err, ErrLockHeld) {
		return nil, err
	}

	if err := recoverStaleLock(path); err != nil {
		return nil, err
	}

	return tryAcquireLock(path)
}

func tryAcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrLockHeld
		}
		return nil, fmt.Errorf("create index lock %s: %w", path, err)
	}

	record := lockRecord{
		PID:       os.Getpid(),
		StartedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("encode index lock record: %w", err)
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write index lock %s: %w", path, err)
	}

	return &Lock{path: path, file: file}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close index lock %s: %w", l.path, err)
	}
	l.file = nil

	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove index lock %s: %w", l.path, err)
	}
	return nil
}

func recoverStaleLock(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read stale index lock %s: %w", path, err)
	}

	record, err := parseLockRecord(data)
	if err != nil {
		return removePossiblyStaleLock(path, fmt.Errorf("parse index lock %s: %w", path, err))
	}

	if processAlive(record.PID) {
		return ErrLockHeld
	}

	return removePossiblyStaleLock(path, nil)
}

func parseLockRecord(data []byte) (lockRecord, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return lockRecord{}, errors.New("empty lock record")
	}

	var record lockRecord
	if err := json.Unmarshal([]byte(trimmed), &record); err == nil && record.PID > 0 {
		return record, nil
	}

	pid, err := strconv.Atoi(trimmed)
	if err != nil || pid <= 0 {
		return lockRecord{}, fmt.Errorf("invalid lock record %q", trimmed)
	}
	return lockRecord{PID: pid}, nil
}

func removePossiblyStaleLock(path string, cause error) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		if cause != nil {
			return fmt.Errorf("%w; remove stale lock: %v", cause, err)
		}
		return fmt.Errorf("remove stale index lock %s: %w", path, err)
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
