package fsutils

import (
	"fmt"
	"io" // Added for io.Copy
	"os"
	"path/filepath" // Added for filepath.Join
	"regexp"        // Needed for sanitization
	"strings"       // Needed for sanitization
)

// CreateDir creates a directory if it doesn't exist.
func CreateDir(path string) error {
	// Implementation needed: Use os.MkdirAll
	fmt.Printf("Placeholder: Would create directory at %s\n", path)
	return os.MkdirAll(path, 0755) // Use standard permission bits
}

// CreateFile creates an empty file. Fails if it already exists.
func CreateFile(path string) error {
	// Implementation needed: Use os.Create (or os.OpenFile with O_CREATE|O_EXCL)
	fmt.Printf("Placeholder: Would create empty file at %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	return f.Close()
}

// WriteToFile writes content to a file, overwriting if it exists.
func WriteToFile(path string, content []byte) error {
	// Implementation needed: Use os.WriteFile
	fmt.Printf("Placeholder: Would write %d bytes to %s\n", len(content), path)
	return os.WriteFile(path, content, 0644) // Standard file permissions
}

// ReadFile reads the content of a file.
func ReadFile(path string) ([]byte, error) {
	// Implementation needed: Use os.ReadFile
	fmt.Printf("Placeholder: Would read file from %s\n", path)
	return os.ReadFile(path)
}

// ScanDir lists files and directories directly under the given path.
func ScanDir(path string) ([]os.DirEntry, error) {
	// Implementation needed: Use os.ReadDir
	fmt.Printf("Placeholder: Would scan directory %s\n", path)
	return os.ReadDir(path)
}

// FileExists checks if a path exists and is a regular file (not a directory).
func FileExists(path string) bool {
	fmt.Printf("Placeholder: Would check if %s exists and is a file\n", path)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false // Path doesn't exist
	}
	if err != nil {
		// Some other error occurred during stat (e.g., permissions)
		// Depending on desired behavior, you might log this or return false.
		// For now, let's return false for any error other than NotExist.
		return false
	}
	// Path exists, now check if it's a regular file
	return !info.IsDir()
}

// CopyDir recursively copies a directory from src to dst.
// It creates the destination directory if it doesn't exist.
// Existing files in the destination will be overwritten.
func CopyDir(src, dst string) error {
	fmt.Printf("Placeholder: Would copy directory from %s to %s\n", src, dst)

	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %q: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	// Create the destination directory
	err = os.MkdirAll(dst, srcInfo.Mode()) // Use source directory's permissions
	if err != nil {
		return fmt.Errorf("failed to create destination directory %q: %w", dst, err)
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %q: %w", src, err)
	}

	// Iterate over entries
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				// Wrap error for better context
				return fmt.Errorf("failed to copy subdirectory %q to %q: %w", srcPath, dstPath, err)
			}
		} else {
			// Copy file
			err = copyFile(srcPath, dstPath)
			if err != nil {
				// Wrap error for better context
				return fmt.Errorf("failed to copy file %q to %q: %w", srcPath, dstPath, err)
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst.
// It overwrites the destination file if it exists.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst) // Creates or truncates
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy data from %q to %q: %w", src, dst, err)
	}

	// Optionally, copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file %q for permissions: %w", src, err)
	}
	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		// Log warning instead of failing? Depends on requirements.
		// log.Printf("Warning: Failed to set permissions on %q: %v", dst, err)
		return fmt.Errorf("failed to set permissions on destination file %q: %w", dst, err)
	}

	return nil
}

// nonAlphanumericRegex matches any character that is NOT a lowercase letter, number, or underscore.
// We also explicitly allow periods for file extensions, though they might be replaced later if needed.
// Note: This regex needs careful consideration based on exact requirements (e.g., allow hyphens?).
var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9_.]+`)
var collapseUnderscoreRegex = regexp.MustCompile(`_+`) // Regex to find consecutive underscores

// SanitizeFilename converts a string into a safe format suitable for filenames or package names.
// It converts to lowercase, replaces spaces and disallowed characters with underscores,
// collapses consecutive underscores, and trims leading/trailing spaces.
func SanitizeFilename(name string) string {
	// 1. Convert to lowercase
	lower := strings.ToLower(name)

	// 2. Trim leading/trailing spaces first
	trimmed := strings.TrimSpace(lower)

	// 3. Replace spaces with underscores
	noSpaces := strings.ReplaceAll(trimmed, " ", "_")

	// 4. Replace all non-alphanumeric characters (except _, .) with underscores
	sanitized := nonAlphanumericRegex.ReplaceAllString(noSpaces, "_")

	// 5. Collapse multiple consecutive underscores into one
	collapsed := collapseUnderscoreRegex.ReplaceAllString(sanitized, "_")

	// 6. REMOVED: Trim leading/trailing underscores
	// final := strings.Trim(collapsed, "_")

	// Re-introduce check for empty result if original input was not empty
	if collapsed == "" && name != "" {
		return "_"
	}

	return collapsed // Return collapsed string directly
}
