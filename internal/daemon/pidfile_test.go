package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := WritePIDFile(path); err != nil {
		t.Fatal(err)
	}

	pid, err := ReadPIDFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
	}
}

func TestReadPIDFile_NotFound(t *testing.T) {
	_, err := ReadPIDFile(filepath.Join(t.TempDir(), "missing.pid"))
	if err == nil {
		t.Error("expected error for missing pid file")
	}
}

func TestReadPIDFile_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	os.WriteFile(path, []byte("not-a-number"), 0644)

	_, err := ReadPIDFile(path)
	if err == nil {
		t.Error("expected error for invalid pid content")
	}
}

func TestRemovePIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	WritePIDFile(path)

	if err := RemovePIDFile(path); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("pid file should be removed")
	}
}

func TestWritePIDFile_CreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "test.pid")

	if err := WritePIDFile(path); err != nil {
		t.Fatal(err)
	}

	pid, err := ReadPIDFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
	}
}

func TestIsRunning_CurrentProcess(t *testing.T) {
	if !IsRunning(os.Getpid()) {
		t.Error("current process should be detected as running")
	}
}

func TestIsRunning_NonExistentProcess(t *testing.T) {
	if IsRunning(99999999) {
		t.Error("non-existent process should not be detected as running")
	}
}
