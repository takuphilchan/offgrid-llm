package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewValidator(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	if validator == nil {
		t.Fatal("NewValidator returned nil")
	}

	if validator.modelsDir != tmpDir {
		t.Errorf("Expected modelsDir %s, got %s", tmpDir, validator.modelsDir)
	}
}

func TestValidateNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	result, err := validator.ValidateModel(filepath.Join(tmpDir, "nonexistent.gguf"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result for nonexistent file")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for nonexistent file")
	}

	t.Logf("Errors (expected): %v", result.Errors)
}

func TestValidateTooSmallFile(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Create a file that's too small
	smallFile := filepath.Join(tmpDir, "small.gguf")
	err := os.WriteFile(smallFile, []byte("tiny"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := validator.ValidateModel(smallFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result for tiny file")
	}

	if !result.IsCorrupted {
		t.Error("Expected IsCorrupted=true for tiny file")
	}

	t.Logf("Errors (expected): %v", result.Errors)
}

func TestValidateInvalidGGUFHeader(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Create a file with wrong magic number but valid size
	invalidFile := filepath.Join(tmpDir, "invalid.gguf")
	data := make([]byte, 2*1024*1024) // 2MB
	copy(data, []byte("FAKE"))        // Wrong magic number
	err := os.WriteFile(invalidFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := validator.ValidateModel(invalidFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result for file with wrong magic number")
	}

	if !result.IsCorrupted {
		t.Error("Expected IsCorrupted=true for wrong magic number")
	}

	if result.IsGGUF {
		t.Error("Expected IsGGUF=false for wrong magic number")
	}

	t.Logf("Errors (expected): %v", result.Errors)
}

func TestValidateValidGGUFHeader(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Create a file with valid GGUF magic number
	validFile := filepath.Join(tmpDir, "valid.gguf")
	data := make([]byte, 2*1024*1024) // 2MB
	copy(data, []byte("GGUF"))        // Valid magic number
	err := os.WriteFile(validFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := validator.ValidateModel(validFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be valid (we only check header, size, and readability)
	if !result.Valid {
		t.Errorf("Expected valid result for GGUF header, got errors: %v", result.Errors)
	}

	if !result.IsGGUF {
		t.Error("Expected IsGGUF=true for valid GGUF header")
	}

	if result.IsCorrupted {
		t.Error("Expected IsCorrupted=false for readable file")
	}

	if result.FileSize != 2*1024*1024 {
		t.Errorf("Expected size 2MB, got %d", result.FileSize)
	}

	if result.SHA256Hash == "" {
		t.Error("Expected SHA256 hash to be calculated")
	}

	t.Logf("Valid GGUF file validated successfully")
	t.Logf("SHA256: %s", result.SHA256Hash)
}

func TestValidateWithExpectedHash(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.gguf")
	data := make([]byte, 2*1024*1024)
	copy(data, []byte("GGUF"))
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First get the actual hash
	result1, err := validator.ValidateModel(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	actualHash := result1.SHA256Hash
	t.Logf("Actual hash: %s", actualHash)

	// Test with correct hash
	result2, err := validator.ValidateWithExpectedHash(testFile, actualHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result2.Valid {
		t.Errorf("Expected valid with correct hash, got errors: %v", result2.Errors)
	}

	// Test with wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	result3, err := validator.ValidateWithExpectedHash(testFile, wrongHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result3.Valid {
		t.Error("Expected invalid with wrong hash")
	}

	if !result3.IsCorrupted {
		t.Error("Expected IsCorrupted=true with hash mismatch")
	}
}

func TestQuickCheck(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Valid file
	validFile := filepath.Join(tmpDir, "valid.gguf")
	data := make([]byte, 2*1024*1024)
	copy(data, []byte("GGUF"))
	err := os.WriteFile(validFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	isValid, err := validator.QuickCheck(validFile)
	if err != nil {
		t.Errorf("QuickCheck failed: %v", err)
	}
	if !isValid {
		t.Error("Expected QuickCheck to return true for valid GGUF")
	}

	// Invalid file
	invalidFile := filepath.Join(tmpDir, "invalid.gguf")
	data2 := make([]byte, 2*1024*1024)
	copy(data2, []byte("FAKE"))
	err = os.WriteFile(invalidFile, data2, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	isValid2, err := validator.QuickCheck(invalidFile)
	if err == nil && isValid2 {
		t.Error("Expected QuickCheck to return false for invalid GGUF")
	}
}

func TestValidateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewValidator(tmpDir)

	// Create multiple test files
	files := map[string]bool{
		"valid1.gguf":  true,  // Valid
		"valid2.gguf":  true,  // Valid
		"invalid.gguf": false, // Invalid magic
		"notgguf.txt":  false, // Will be skipped
		"tiny.gguf":    false, // Too small
	}

	for filename, shouldBeValid := range files {
		path := filepath.Join(tmpDir, filename)
		var data []byte

		if filename == "tiny.gguf" {
			data = []byte("tiny")
		} else if shouldBeValid {
			data = make([]byte, 2*1024*1024)
			copy(data, []byte("GGUF"))
		} else if filename != "notgguf.txt" {
			data = make([]byte, 2*1024*1024)
			copy(data, []byte("FAKE"))
		} else {
			data = []byte("not a gguf file")
		}

		err := os.WriteFile(path, data, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	results, err := validator.ValidateDirectory()
	if err != nil {
		t.Fatalf("ValidateDirectory failed: %v", err)
	}

	// Should only validate .gguf files
	if len(results) != 4 { // 4 .gguf files
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Check each result
	for filename, result := range results {
		t.Logf("%s: Valid=%v, Errors=%v", filename, result.Valid, result.Errors)
	}

	// valid1.gguf and valid2.gguf should be valid
	if result, ok := results["valid1.gguf"]; ok {
		if !result.Valid {
			t.Errorf("valid1.gguf should be valid, errors: %v", result.Errors)
		}
	}
}

func TestValidateIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid test file
	testFile := filepath.Join(tmpDir, "test.gguf")
	data := make([]byte, 2*1024*1024)
	copy(data, []byte("GGUF"))
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get actual hash
	validator := NewValidator(tmpDir)
	result, _ := validator.ValidateModel(testFile)
	actualHash := result.SHA256Hash

	// Test with correct hash
	err = ValidateIntegrity(testFile, actualHash)
	if err != nil {
		t.Errorf("ValidateIntegrity failed with correct hash: %v", err)
	}

	// Test with wrong hash
	err = ValidateIntegrity(testFile, "wrong")
	if err == nil {
		t.Error("ValidateIntegrity should fail with wrong hash")
	}
}

func BenchmarkValidateModel(b *testing.B) {
	tmpDir := b.TempDir()
	validator := NewValidator(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "bench.gguf")
	data := make([]byte, 10*1024*1024) // 10MB
	copy(data, []byte("GGUF"))
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateModel(testFile)
	}
}

func BenchmarkQuickCheck(b *testing.B) {
	tmpDir := b.TempDir()
	validator := NewValidator(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "bench.gguf")
	data := make([]byte, 10*1024*1024) // 10MB
	copy(data, []byte("GGUF"))
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.QuickCheck(testFile)
	}
}
