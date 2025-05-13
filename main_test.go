package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	if config.AutoRename != false {
		t.Errorf("Default AutoRename = %v, want false", config.AutoRename)
	}
	if config.CustomPrompt != defaultPrompt {
		t.Errorf("Default CustomPrompt = %q, want %q", config.CustomPrompt, defaultPrompt)
	}
	if config.Model != "gemma3:1b" {
		t.Errorf("Default Model = %q, want gemma3:1b", config.Model)
	}
}

func TestFlagParsing(t *testing.T) {
	// Save original flag.CommandLine and restore after test
	originalFlagCommandLine := flag.CommandLine
	defer func() { flag.CommandLine = originalFlagCommandLine }()

	// Test that flags override defaults
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	defaultConfig := getDefaultConfig()
	autoRename := flag.Bool("auto", defaultConfig.AutoRename, "")
	customPrompt := flag.String("prompt", defaultConfig.CustomPrompt, "")
	model := flag.String("model", defaultConfig.Model, "")

	// Test with custom values
	os.Args = []string{"test", "-auto", "-prompt", "custom prompt", "-model", "llama2"}
	flag.Parse()

	got := Config{
		AutoRename:   *autoRename,
		CustomPrompt: *customPrompt,
		Model:        *model,
	}

	want := Config{
		AutoRename:   true,
		CustomPrompt: "custom prompt",
		Model:        "llama2",
	}

	if got != want {
		t.Errorf("Flag parsing failed:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestUsageDisplay(t *testing.T) {
	// Save original stdout and stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	// Create pipes to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	// Save original args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Test with no arguments
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Run main() which should display usage
	go func() {
		main()
		w.Close()
	}()

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check for expected usage elements
	expectedElements := []string{
		"Usage:",
		"Options:",
		"-auto",
		"-prompt",
		"-model",
		"Examples:",
		"*.pdf",
		"file1.pdf file2.pdf",
		"custom prompt",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Usage output missing expected element: %q", element)
		}
	}
}

func TestDependencyMessages(t *testing.T) {
	// Test that error messages for missing dependencies are clear and helpful
	expectedMessages := map[string]string{
		"ocrmypdf": "ocrmypdf is not installed",
		"curl":     "curl is not installed",
		"jq":       "jq is not installed",
		"ollama":   "ollama is not installed",
	}

	for dep, expectedMsg := range expectedMessages {
		t.Run(dep, func(t *testing.T) {
			// Create a temporary file to simulate the dependency
			tmpFile := filepath.Join(t.TempDir(), dep)
			if err := os.WriteFile(tmpFile, []byte(""), 0755); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Add the temporary directory to PATH
			originalPath := os.Getenv("PATH")
			os.Setenv("PATH", filepath.Dir(tmpFile))
			defer os.Setenv("PATH", originalPath)

			// Remove the file to simulate missing dependency
			os.Remove(tmpFile)

			// Check dependencies
			err := checkDependencies()

			// Verify error message
			if err == nil {
				t.Errorf("Expected error for missing %s, got nil", dep)
				return
			}

			if !strings.Contains(err.Error(), expectedMsg) {
				t.Errorf("Error message for %s = %q, want message containing %q", dep, err.Error(), expectedMsg)
			}
		})
	}
}
