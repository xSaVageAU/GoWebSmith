package fsutils

import (
	"bytes"
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

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()

	// Test 1: File that exists
	filePath := filepath.Join(tempDir, "exists.txt")
	// Create an empty file for testing existence
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Test 1 setup failed: Could not create temp file %q: %v", filePath, err)
	}
	file.Close() // Close the file immediately

	if !FileExists(filePath) {
		t.Errorf("Test 1 failed: FileExists(%q) returned false, want true", filePath)
	}

	// Test 2: File that does not exist
	nonExistentPath := filepath.Join(tempDir, "does_not_exist.txt")
	if FileExists(nonExistentPath) {
		t.Errorf("Test 2 failed: FileExists(%q) returned true, want false", nonExistentPath)
	}

	// Test 3: Path is a directory, not a file
	dirPath := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Test 3 setup failed: Could not create temp subdir %q: %v", dirPath, err)
	}
	if FileExists(dirPath) {
		t.Errorf("Test 3 failed: FileExists(%q) on a directory returned true, want false", dirPath)
	}

	// Test 4: Path is empty string
	if FileExists("") {
		t.Errorf("Test 4 failed: FileExists(\"\") returned true, want false")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Spaces", "My Module Name", "my_module_name"},
		{"Special Chars", "Module!@#$%^&*()_+=", "module_"}, // Collapsed, no final trim
		{"Already Valid", "valid_name_123", "valid_name_123"},
		{"Mixed Case", "SomeMixed_Case", "somemixed_case"},
		{"Leading/Trailing Spaces", "  leading and trailing  ", "leading_and_trailing"},
		{"Consecutive Special Chars", "a!!b@#c", "a_b_c"}, // Collapsed
		{"Empty String", "", ""},
		{"Only Special Chars", "!@#$", "_"}, // Collapsed, becomes "_"
		{"Starts with Number", "1st_module", "1st_module"},
		{"Unicode (basic test)", "你好世界", "_"}, // Collapsed, becomes "_"
		{"With Periods", "file.name.ext", "file.name.ext"},
		{"Leading Underscores", "__dunder__", "_dunder_"},   // No final trim
		{"Trailing Underscores", "trailing__", "trailing_"}, // No final trim
		{"Spaces and Special", " a ! b ", "a_b"},            // Corrected expectation: Trimmed space, replaced !, collapsed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCopyDir(t *testing.T) {
	tempDir := t.TempDir()

	// --- Base Case: Copy directory with files and subdirs ---
	t.Run("BaseCopy", func(t *testing.T) {
		// Source directory structure
		srcDir := filepath.Join(tempDir, "source_base")
		subDir := filepath.Join(srcDir, "subdir")
		file1Path := filepath.Join(srcDir, "file1.txt")
		file2Path := filepath.Join(subDir, "file2.txt")

		// Create source directories and files
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Setup failed: Could not create source directories: %v", err)
		}
		content1 := []byte("Content of file 1")
		content2 := []byte("Content of file 2 in subdir")
		if err := os.WriteFile(file1Path, content1, 0644); err != nil {
			t.Fatalf("Setup failed: Could not write source file1: %v", err)
		}
		if err := os.WriteFile(file2Path, content2, 0644); err != nil {
			t.Fatalf("Setup failed: Could not write source file2: %v", err)
		}

		// Destination directory path
		dstDir := filepath.Join(tempDir, "destination_base")

		// Execute
		err := CopyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("CopyDir(%q, %q) failed: %v", srcDir, dstDir, err)
		}

		// Verification
		dstSubDir := filepath.Join(dstDir, "subdir")
		if _, err := os.Stat(dstDir); os.IsNotExist(err) {
			t.Errorf("Verification failed: Destination directory %q was not created", dstDir)
		}
		if _, err := os.Stat(dstSubDir); os.IsNotExist(err) {
			t.Errorf("Verification failed: Destination subdirectory %q was not created", dstSubDir)
		}
		dstFile1Path := filepath.Join(dstDir, "file1.txt")
		dstFile2Path := filepath.Join(dstSubDir, "file2.txt")
		if _, err := os.Stat(dstFile1Path); os.IsNotExist(err) {
			t.Errorf("Verification failed: Destination file %q was not created", dstFile1Path)
		}
		if _, err := os.Stat(dstFile2Path); os.IsNotExist(err) {
			t.Errorf("Verification failed: Destination file %q was not created", dstFile2Path)
		}
		readContent1, err := os.ReadFile(dstFile1Path)
		if err != nil {
			t.Errorf("Verification failed: Could not read destination file %q: %v", dstFile1Path, err)
		} else if !bytes.Equal(readContent1, content1) {
			t.Errorf("Verification failed: Content mismatch for %q. Got %q, want %q", dstFile1Path, string(readContent1), string(content1))
		}
		readContent2, err := os.ReadFile(dstFile2Path)
		if err != nil {
			t.Errorf("Verification failed: Could not read destination file %q: %v", dstFile2Path, err)
		} else if !bytes.Equal(readContent2, content2) {
			t.Errorf("Verification failed: Content mismatch for %q. Got %q, want %q", dstFile2Path, string(readContent2), string(content2))
		}
	})

	// --- Case: Empty source directory ---
	t.Run("EmptySource", func(t *testing.T) {
		srcDir := filepath.Join(tempDir, "source_empty")
		if err := os.Mkdir(srcDir, 0755); err != nil {
			t.Fatalf("Setup failed: Could not create empty source dir: %v", err)
		}
		dstDir := filepath.Join(tempDir, "destination_empty")

		err := CopyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("CopyDir failed for empty source: %v", err)
		}

		if _, err := os.Stat(dstDir); os.IsNotExist(err) {
			t.Errorf("Verification failed: Destination directory %q was not created", dstDir)
		}
		entries, err := os.ReadDir(dstDir)
		if err != nil {
			t.Errorf("Verification failed: Could not read destination dir %q: %v", dstDir, err)
		} else if len(entries) != 0 {
			t.Errorf("Verification failed: Destination directory %q is not empty, contains %d entries", dstDir, len(entries))
		}
	})

	// --- Case: Source does not exist ---
	t.Run("SourceNotExist", func(t *testing.T) {
		srcDir := filepath.Join(tempDir, "source_nonexistent")
		dstDir := filepath.Join(tempDir, "destination_nonexistent")

		err := CopyDir(srcDir, dstDir)
		if err == nil {
			t.Fatalf("CopyDir succeeded for non-existent source, expected error")
		}
		// Check if the error indicates the source doesn't exist (optional, depends on error wrapping)
		t.Logf("Received expected error for non-existent source: %v", err)
	})

	// --- Case: Source is a file ---
	t.Run("SourceIsFile", func(t *testing.T) {
		srcFile := filepath.Join(tempDir, "source_is_file.txt")
		if err := os.WriteFile(srcFile, []byte("i am a file"), 0644); err != nil {
			t.Fatalf("Setup failed: Could not create source file: %v", err)
		}
		dstDir := filepath.Join(tempDir, "destination_source_is_file")

		err := CopyDir(srcFile, dstDir)
		if err == nil {
			t.Fatalf("CopyDir succeeded when source is a file, expected error")
		}
		// Check if the error indicates source is not a directory (optional)
		t.Logf("Received expected error when source is a file: %v", err)
	})

	// --- Case: Destination already exists (overwrite) ---
	t.Run("DestinationExistsOverwrite", func(t *testing.T) {
		// Source
		srcDir := filepath.Join(tempDir, "source_overwrite")
		file1Path := filepath.Join(srcDir, "file1.txt")
		if err := os.Mkdir(srcDir, 0755); err != nil {
			t.Fatalf("Setup failed: Could not create source dir: %v", err)
		}
		contentNew := []byte("New Content")
		if err := os.WriteFile(file1Path, contentNew, 0644); err != nil {
			t.Fatalf("Setup failed: Could not write source file: %v", err)
		}

		// Destination with pre-existing file
		dstDir := filepath.Join(tempDir, "destination_overwrite")
		dstFile1Path := filepath.Join(dstDir, "file1.txt")
		if err := os.Mkdir(dstDir, 0755); err != nil {
			t.Fatalf("Setup failed: Could not create destination dir: %v", err)
		}
		contentOld := []byte("Old Content")
		if err := os.WriteFile(dstFile1Path, contentOld, 0644); err != nil {
			t.Fatalf("Setup failed: Could not write initial destination file: %v", err)
		}

		// Execute
		err := CopyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("CopyDir failed for overwrite scenario: %v", err)
		}

		// Verification
		readContent, err := os.ReadFile(dstFile1Path)
		if err != nil {
			t.Errorf("Verification failed: Could not read overwritten destination file %q: %v", dstFile1Path, err)
		} else if !bytes.Equal(readContent, contentNew) {
			t.Errorf("Verification failed: File %q was not overwritten. Got %q, want %q", dstFile1Path, string(readContent), string(contentNew))
		}
	})
}
