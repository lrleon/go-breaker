package breaker

import (
	"os"
	"testing"
)

// Test normal
func TestK8SGetMemoryLimit(t *testing.T) {

	// create a temporary file to simulate the memory limit file
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(memoryLimitFile)
	memoryLimitFile = "memory.limit_in_bytes"
	file, err := os.Create(memoryLimitFile)
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	_, err = file.WriteString("536870912")
	if err != nil {
		return
	} // 512 MB

	limit, err := getK8sMemoryLimit()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := int64(536870912)
	if limit != expected {
		t.Errorf("Expected %d, got %d", expected, limit)
	}
}

// Test con error simulado
func TestGetMemoryLimit_Error(t *testing.T) {

	// create a temporary file to simulate the memory limit file
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(memoryLimitFile)
	memoryLimitFile = "memory.limit_in_bytes"
	file, err := os.Create(memoryLimitFile)
	if err != nil {
		t.Fatalf("Error creating file: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	_, err = file.WriteString("abc")
	if err != nil {
		return
	} // invalid value

	_, err = getK8sMemoryLimit()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
