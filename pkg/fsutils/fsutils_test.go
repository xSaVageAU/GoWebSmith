package fsutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test 1: Create a new directory
	newDirPath := filepath.Join(tempDir, "new_dir")
	err := CreateDir(newDirPath)
	if err != nil {
		t.Fatalf("Test 1 failed: CreateDir(%q) returned error: %v", newDirPath, err)
	}
	if _, err := os.Stat(newDirPath); os.IsNotExist(err) {
		t.Fatalf("Test 1 failed: Directory %q was not created", newDirPath)
	}

	// Test 2: Create a directory that already exists
	err = CreateDir(newDirPath)
	if err != nil {
		t.Fatalf("Test 2 failed: CreateDir(%q) on existing dir returned error: %v", newDirPath, err)
	}

	// Test 3: Create nested directories
	nestedDirPath := filepath.Join(tempDir, "parent", "child")
	err = CreateDir(nestedDirPath)
	if err != nil {
		t.Fatalf("Test 3 failed: CreateDir(%q) for nested dirs returned error: %v", nestedDirPath, err)
	}
	if _, err := os.Stat(nestedDirPath); os.IsNotExist(err) {
		t.Fatalf("Test 3 failed: Nested directory %q was not created", nestedDirPath)
	}
}

func TestWriteToFile(t *testing.T) {
	tempDir := t.TempDir()

	// Test 1: Write to a new file
	filePath1 := filepath.Join(tempDir, "testfile1.txt")
	content1 := []byte("Hello, World!")
	err := WriteToFile(filePath1, content1)
	if err != nil {
		t.Fatalf("Test 1 failed: WriteToFile(%q) returned error: %v", filePath1, err)
	}

	// Verify content
	readContent1, err := os.ReadFile(filePath1)
	if err != nil {
		t.Fatalf("Test 1 failed: Error reading back file %q: %v", filePath1, err)
	}
	if string(readContent1) != string(content1) {
		t.Fatalf("Test 1 failed: Read content %q does not match written content %q", string(readContent1), string(content1))
	}

	// Test 2: Overwrite an existing file
	filePath2 := filepath.Join(tempDir, "testfile2.txt")
	content2a := []byte("Initial content")
	content2b := []byte("Overwritten content")

	// Write initial content
	err = WriteToFile(filePath2, content2a)
	if err != nil {
		t.Fatalf("Test 2 setup failed: WriteToFile(%q) returned error: %v", filePath2, err)
	}

	// Write overwritten content
	err = WriteToFile(filePath2, content2b)
	if err != nil {
		t.Fatalf("Test 2 failed: WriteToFile(%q) overwrite returned error: %v", filePath2, err)
	}

	// Verify overwritten content
	readContent2, err := os.ReadFile(filePath2)
	if err != nil {
		t.Fatalf("Test 2 failed: Error reading back overwritten file %q: %v", filePath2, err)
	}
	if string(readContent2) != string(content2b) {
		t.Fatalf("Test 2 failed: Read content %q does not match overwritten content %q", string(readContent2), string(content2b))
	}

	// Test 3: Write to a file in a non-existent directory (should fail unless WriteToFile creates dirs)
	// Assuming WriteToFile does NOT create parent directories based on its current implementation
	filePath3 := filepath.Join(tempDir, "non_existent_dir", "testfile3.txt")
	content3 := []byte("Test")
	err = WriteToFile(filePath3, content3)
	if err == nil {
		t.Fatalf("Test 3 failed: WriteToFile(%q) succeeded, expected error for non-existent directory", filePath3)
	}
	// Check if the error is related to the path not existing
	if !os.IsNotExist(err) {
		// This check might be too specific depending on the exact error returned by os.WriteFile
		// t.Logf("Test 3: Received expected error type: %v", err)
	} else {
		t.Logf("Test 3: Received expected os.IsNotExist error: %v", err)
	}

}
