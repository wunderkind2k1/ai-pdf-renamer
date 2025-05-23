package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockExitor implements Exitor for testing purposes
type MockExitor struct {
	ExitCalled bool
	ExitCode   int
}

func (m *MockExitor) Exit(code int) {
	m.ExitCalled = true
	m.ExitCode = code
}

// TestDefaultConfig verifies that the default configuration has the expected values
func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name     string
		check    func(Config) bool
		expected bool
		message  string
	}{
		{
			name: "AutoRename default",
			check: func(c Config) bool {
				return c.AutoRename == false
			},
			expected: true,
			message:  "Default AutoRename should be false",
		},
		{
			name: "CustomPrompt default",
			check: func(c Config) bool {
				return c.CustomPrompt == defaultPrompt
			},
			expected: true,
			message:  "Default CustomPrompt should match defaultPrompt",
		},
		{
			name: "Model default",
			check: func(c Config) bool {
				return c.Model == "qwen2.5vl:7b"
			},
			expected: true,
			message:  "Default Model should be qwen2.5vl:7b",
		},
		{
			name: "FastMode default",
			check: func(c Config) bool {
				return c.FastMode == true
			},
			expected: true,
			message:  "Default FastMode should be true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := getDefaultConfig()
			if got := tt.check(config); got != tt.expected {
				t.Error(tt.message)
			}
		})
	}
}

// TestFlagParsing verifies that command line flags are correctly parsed
func TestFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "All flags set",
			args: []string{"test", "-auto", "-prompt", "custom prompt", "-model", "llama2", "-novision"},
			expected: Config{
				AutoRename:   true,
				CustomPrompt: "custom prompt",
				Model:        "llama2",
				FastMode:     false,
			},
		},
		{
			name: "No flags (defaults)",
			args: []string{"test"},
			expected: Config{
				AutoRename:   false,
				CustomPrompt: defaultPrompt,
				Model:        "qwen2.5vl:7b",
				FastMode:     true,
			},
		},
		{
			name: "Only auto flag",
			args: []string{"test", "-auto"},
			expected: Config{
				AutoRename:   true,
				CustomPrompt: defaultPrompt,
				Model:        "qwen2.5vl:7b",
				FastMode:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original flag.CommandLine
			originalFlagCommandLine := flag.CommandLine
			defer func() { flag.CommandLine = originalFlagCommandLine }()

			// Set up new flag set
			flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
			defaultConfig := getDefaultConfig()
			autoRename := flag.Bool("auto", defaultConfig.AutoRename, "")
			customPrompt := flag.String("prompt", defaultConfig.CustomPrompt, "")
			model := flag.String("model", defaultConfig.Model, "")
			noVision := flag.Bool("novision", false, "")

			// Set test args and parse
			os.Args = tt.args
			flag.Parse()

			got := Config{
				AutoRename:   *autoRename,
				CustomPrompt: *customPrompt,
				Model:        *model,
				FastMode:     !*noVision,
			}

			if got != tt.expected {
				t.Errorf("Flag parsing failed:\ngot:  %+v\nwant: %+v", got, tt.expected)
			}
		})
	}
}

// TestUsageDisplay_Ignored verifies that the usage information is correctly displayed
func TestUsageDisplay_Ignored(t *testing.T) {
	t.Skip("TestUsageDisplay is ignored for now.")
	// Save and restore original stdout/stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	// Create pipes to capture output
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Save and restore original flag.CommandLine
	originalFlagCommandLine := flag.CommandLine
	defer func() {
		flag.CommandLine = originalFlagCommandLine
	}()

	// Set up a dummy config (using a MockExitor) so that setup() uses the custom flag.Usage from main.go
	cfg := Config{Exitor: &MockExitor{}}
	// Call setup() (which sets up flags and calls flag.Usage) instead of calling flag.Usage() directly
	setup(cfg)

	// Close pipes and read output
	stdoutW.Close()
	stderrW.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutBuf.ReadFrom(stdoutR)
	stderrBuf.ReadFrom(stderrR)
	output := stdoutBuf.String() + stderrBuf.String()

	// Only check for the main sections and flag names
	requiredElements := []string{
		"Usage of",
		"Options:",
		"-auto",
		"-model",
		"-novision",
		"-output",
		"-prompt",
		"Examples:",
		"Note: Vision-based processing is enabled by default.",
	}

	for _, element := range requiredElements {
		if !strings.Contains(output, element) {
			t.Errorf("Usage output missing required element: %q", element)
		}
	}
}

// TestDependencyChecking verifies that dependency checks work correctly
func TestDependencyChecking(t *testing.T) {
	t.Skip("TestDependencyChecking is ignored for now.")
	tests := []struct {
		name         string
		dependency   string
		expectedMsg  string
		setupTestDir func(string) func()
	}{
		{
			name:        "Missing ocrmypdf",
			dependency:  "ocrmypdf",
			expectedMsg: "ocrmypdf is not installed",
			setupTestDir: func(tmpDir string) func() {
				// Create a mock curl executable to prevent early exit
				curlPath := filepath.Join(tmpDir, "curl")
				os.WriteFile(curlPath, []byte("#!/bin/sh\necho 'mock curl'"), 0755)
				originalPath := os.Getenv("PATH")
				os.Setenv("PATH", tmpDir)
				return func() { os.Setenv("PATH", originalPath) }
			},
		},
		{
			name:        "Missing curl",
			dependency:  "curl",
			expectedMsg: "curl is not installed",
			setupTestDir: func(tmpDir string) func() {
				// Create a mock ocrmypdf executable to prevent early exit
				ocrPath := filepath.Join(tmpDir, "ocrmypdf")
				os.WriteFile(ocrPath, []byte("#!/bin/sh\necho 'mock ocrmypdf'"), 0755)
				originalPath := os.Getenv("PATH")
				os.Setenv("PATH", tmpDir)
				return func() { os.Setenv("PATH", originalPath) }
			},
		},
		{
			name:        "Missing jq",
			dependency:  "jq",
			expectedMsg: "jq is not installed",
			setupTestDir: func(tmpDir string) func() {
				// Create mock executables for required dependencies
				for _, dep := range []string{"curl", "ocrmypdf"} {
					depPath := filepath.Join(tmpDir, dep)
					os.WriteFile(depPath, []byte("#!/bin/sh\necho 'mock "+dep+"'"), 0755)
				}
				originalPath := os.Getenv("PATH")
				os.Setenv("PATH", tmpDir)
				return func() { os.Setenv("PATH", originalPath) }
			},
		},
		{
			name:        "Missing ollama",
			dependency:  "ollama",
			expectedMsg: "ollama is not installed",
			setupTestDir: func(tmpDir string) func() {
				// Create mock executables for required dependencies
				for _, dep := range []string{"curl", "ocrmypdf", "jq"} {
					depPath := filepath.Join(tmpDir, dep)
					os.WriteFile(depPath, []byte("#!/bin/sh\necho 'mock "+dep+"'"), 0755)
				}
				originalPath := os.Getenv("PATH")
				os.Setenv("PATH", tmpDir)
				return func() { os.Setenv("PATH", originalPath) }
			},
		},
		{
			name:        "Missing gs",
			dependency:  "gs",
			expectedMsg: "gs is not installed",
			setupTestDir: func(tmpDir string) func() {
				// Create mock executables for required dependencies
				for _, dep := range []string{"curl", "ocrmypdf", "jq", "ollama"} {
					depPath := filepath.Join(tmpDir, dep)
					os.WriteFile(depPath, []byte("#!/bin/sh\necho 'mock "+dep+"'"), 0755)
				}
				originalPath := os.Getenv("PATH")
				os.Setenv("PATH", tmpDir)
				return func() { os.Setenv("PATH", originalPath) }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tmpDir := t.TempDir()
			cleanup := tt.setupTestDir(tmpDir)
			defer cleanup()

			// Create and remove test file to simulate missing dependency
			tmpFile := filepath.Join(tmpDir, tt.dependency)
			if err := os.WriteFile(tmpFile, []byte(""), 0755); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			os.Remove(tmpFile)

			// Check dependencies
			err := checkDependencies()

			// Verify error
			if err == nil {
				t.Error("Expected error for missing dependency, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedMsg) {
				t.Errorf("Error message = %q, want message containing %q", err.Error(), tt.expectedMsg)
			}
		})
	}
}

// TestModelSwitching verifies that model selection works correctly based on vision mode
func TestModelSwitching(t *testing.T) {
	tests := []struct {
		name          string
		initialModel  string
		useVision     bool
		expectedModel string
	}{
		{
			name:          "Vision mode with llama2",
			initialModel:  "llama2",
			useVision:     true,
			expectedModel: "qwen2.5vl:7b",
		},
		{
			name:          "No vision mode with llama2",
			initialModel:  "llama2",
			useVision:     false,
			expectedModel: "llama2",
		},
		{
			name:          "Vision mode with default model",
			initialModel:  "qwen2.5vl:7b",
			useVision:     true,
			expectedModel: "qwen2.5vl:7b",
		},
		{
			name:          "No vision mode with default model",
			initialModel:  "qwen2.5vl:7b",
			useVision:     false,
			expectedModel: "qwen2.5vl:7b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original flag.CommandLine
			originalFlagCommandLine := flag.CommandLine
			defer func() { flag.CommandLine = originalFlagCommandLine }()

			// Set up flags
			flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
			model := flag.String("model", tt.initialModel, "")
			noVision := flag.Bool("novision", !tt.useVision, "")

			// Set args and parse
			args := []string{"test", "-model", tt.initialModel}
			if !tt.useVision {
				args = append(args, "-novision")
			}
			os.Args = args
			flag.Parse()

			// Create config and run setup
			cfg := Config{
				Model:    *model,
				FastMode: !*noVision,
				Exitor:   &MockExitor{},
			}

			// Run setup to trigger model switching
			setup(cfg)

			// Verify model
			if config.Model != tt.expectedModel {
				t.Errorf("Model = %q, want %q", config.Model, tt.expectedModel)
			}
		})
	}
}
