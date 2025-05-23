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
	if config.Model != "qwen2.5vl:7b" {
		t.Errorf("Default Model = %q, want qwen2.5vl:7b", config.Model)
	}
	if config.FastMode != true {
		t.Errorf("Default FastMode = %v, want true", config.FastMode)
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
	noVision := flag.Bool("novision", false, "")

	// Test with custom values
	os.Args = []string{"test", "-auto", "-prompt", "custom prompt", "-model", "llama2", "-novision"}
	flag.Parse()

	got := Config{
		AutoRename:   *autoRename,
		CustomPrompt: *customPrompt,
		Model:        *model,
		FastMode:     !*noVision, // Invert novision flag to get FastMode
	}

	want := Config{
		AutoRename:   true,
		CustomPrompt: "custom prompt",
		Model:        "llama2",
		FastMode:     false, // novision flag is true, so FastMode should be false
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
		"-novision",
		"Examples:",
		"*.pdf",
		"file1.pdf file2.pdf",
		"custom prompt",
		"Vision-based processing is enabled by default",
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
		"gs":       "gs is not installed",
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

func TestModelSwitching(t *testing.T) {
	// Test that model is automatically switched to qwen2.5vl:7b when vision mode is enabled
	config = Config{
		Model:    "llama2",
		FastMode: true,
	}

	// Initialize config with defaults
	defaultConfig := getDefaultConfig()
	model := flag.String("model", defaultConfig.Model, "")
	noVision := flag.Bool("novision", false, "")

	// Test with vision mode enabled (default)
	os.Args = []string{"test", "-model", "llama2"}
	flag.Parse()

	config = Config{
		Model:    *model,
		FastMode: !*noVision,
	}

	// Model should be switched to qwen2.5vl:7b
	if config.Model != "qwen2.5vl:7b" {
		t.Errorf("Model not switched to qwen2.5vl:7b in vision mode, got %q", config.Model)
	}

	// Test with vision mode disabled
	os.Args = []string{"test", "-model", "llama2", "-novision"}
	flag.Parse()

	config = Config{
		Model:    *model,
		FastMode: !*noVision,
	}

	// Model should remain as llama2
	if config.Model != "llama2" {
		t.Errorf("Model incorrectly switched in OCR mode, got %q", config.Model)
	}
}
