package filesystems

import (
	"testing"
	"github.com/railwayapp/turnout/internal/filesystems"
)

func TestMemoryFS_AddFile(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	content := []byte("hello world")
	mfs.AddFile("test.txt", content)

	result, err := mfs.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(result) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(result))
	}
}

func TestMemoryFS_AddFile_CreatesParentDirs(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	mfs.AddFile("dir1/dir2/test.txt", []byte("content"))

	// Verify parent directories exist by reading the file
	content, err := mfs.ReadFile("dir1/dir2/test.txt")
	if err != nil {
		t.Fatalf("expected no error reading file in nested directory, got %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got '%s'", string(content))
	}
}

func TestMemoryFS_ReadFile_NotFound(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	
	_, err := mfs.ReadFile("nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestMemoryFS_ReadDir(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	mfs.AddFile("file1.txt", []byte("content1"))
	mfs.AddFile("file2.txt", []byte("content2"))
	mfs.AddDir("subdir")
	mfs.AddFile("subdir/file3.txt", []byte("content3"))

	entries := make([]string, 0)
	for entry, err := range mfs.ReadDir(".") {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entries = append(entries, entry.Name())
	}

	expected := []string{"file1.txt", "file2.txt", "subdir"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}

	for i, name := range expected {
		if entries[i] != name {
			t.Errorf("expected entry %d to be '%s', got '%s'", i, name, entries[i])
		}
	}
}

func TestMemoryFS_ReadDir_Subdirectory(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	mfs.AddFile("dir/file1.txt", []byte("content1"))
	mfs.AddFile("dir/file2.txt", []byte("content2"))

	entries := make([]string, 0)
	for entry, err := range mfs.ReadDir("dir") {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entries = append(entries, entry.Name())
	}

	expected := []string{"file1.txt", "file2.txt"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}

	for i, name := range expected {
		if entries[i] != name {
			t.Errorf("expected entry %d to be '%s', got '%s'", i, name, entries[i])
		}
	}
}

func TestMemoryFS_Walk(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	mfs.AddFile("file1.txt", []byte("content1"))
	mfs.AddFile("dir1/file2.txt", []byte("content2"))
	mfs.AddFile("dir1/dir2/file3.txt", []byte("content3"))

	visited := make([]string, 0)
	err := mfs.Walk(".", func(path string, info filesystems.FileInfo, err error) error {
		if err != nil {
			return err
		}
		visited = append(visited, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that we visited all expected paths
	expectedPaths := []string{".", "file1.txt", "dir1", "dir1/file2.txt", "dir1/dir2", "dir1/dir2/file3.txt"}
	if len(visited) < len(expectedPaths) {
		t.Fatalf("expected at least %d paths, got %d", len(expectedPaths), len(visited))
	}

	// Check that key paths are present
	pathSet := make(map[string]bool)
	for _, p := range visited {
		pathSet[p] = true
	}

	for _, expectedPath := range expectedPaths {
		if !pathSet[expectedPath] {
			t.Errorf("expected to visit path '%s', but didn't", expectedPath)
		}
	}
}

func TestMemoryFS_PathOperations(t *testing.T) {
	mfs := filesystems.NewMemoryFS()

	// Test Join
	joined := mfs.Join("dir", "subdir", "file.txt")
	expected := "dir/subdir/file.txt"
	if joined != expected {
		t.Errorf("expected Join to return '%s', got '%s'", expected, joined)
	}

	// Test Base
	base := mfs.Base("dir/subdir/file.txt")
	expected = "file.txt"
	if base != expected {
		t.Errorf("expected Base to return '%s', got '%s'", expected, base)
	}

	// Test Dir
	dir := mfs.Dir("dir/subdir/file.txt")
	expected = "dir/subdir"
	if dir != expected {
		t.Errorf("expected Dir to return '%s', got '%s'", expected, dir)
	}
}

func TestMemoryFS_Rel(t *testing.T) {
	mfs := filesystems.NewMemoryFS()

	// Test same paths
	rel, err := mfs.Rel("dir", "dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != "." {
		t.Errorf("expected '.', got '%s'", rel)
	}

	// Test relative path
	rel, err = mfs.Rel("dir", "dir/subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != "subdir/file.txt" {
		t.Errorf("expected 'subdir/file.txt', got '%s'", rel)
	}
}

func TestMemoryFS_DirEntry_Info(t *testing.T) {
	mfs := filesystems.NewMemoryFS()
	mfs.AddFile("test.txt", []byte("hello world"))
	mfs.AddDir("testdir")

	// Test file entry info
	for entry, err := range mfs.ReadDir(".") {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		
		if entry.Name() == "test.txt" {
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("unexpected error getting file info: %v", err)
			}
			
			if info.Name() != "test.txt" {
				t.Errorf("expected name 'test.txt', got '%s'", info.Name())
			}
			
			if info.Size() != 11 {
				t.Errorf("expected size 11, got %d", info.Size())
			}
			
			if info.IsDir() {
				t.Error("expected file to not be directory")
			}
		}
		
		if entry.Name() == "testdir" {
			if !entry.IsDir() {
				t.Error("expected directory entry to report as directory")
			}
			
			info, err := entry.Info()
			if err != nil {
				t.Fatalf("unexpected error getting dir info: %v", err)
			}
			
			if !info.IsDir() {
				t.Error("expected directory info to report as directory")
			}
		}
	}
}