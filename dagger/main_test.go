package main

import (
	"path/filepath"
	"testing"
)

func TestBuildPlatforms(t *testing.T) {
	// Define expected platforms
	expectedPlatforms := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}

	// Verify platforms match exactly
	if len(BuildPlatforms) != len(expectedPlatforms) {
		t.Errorf("Expected %d platforms, got %d", len(expectedPlatforms), len(BuildPlatforms))
	}

	// Create a map for easier comparison
	platformMap := make(map[string]bool)
	for _, p := range BuildPlatforms {
		key := p.os + "/" + p.arch
		platformMap[key] = true
	}

	// Check each expected platform
	for _, p := range expectedPlatforms {
		key := p.os + "/" + p.arch
		if !platformMap[key] {
			t.Errorf("Expected platform %s not found in build configuration", key)
		}
	}

	// Check for unexpected platforms
	for _, p := range BuildPlatforms {
		key := p.os + "/" + p.arch
		found := false
		for _, ep := range expectedPlatforms {
			if ep.os == p.os && ep.arch == p.arch {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected platform found in build configuration: %s", key)
		}
	}
}

func TestGetOutputDir(t *testing.T) {
	const expectedDir = "build"

	// Get the actual output directory path
	actualOutputDir, err := getOutputDir()
	if err != nil {
		t.Fatalf("Error getting output directory: %v", err)
	}

	// Get the base name of the actual path
	actualBase := filepath.Base(actualOutputDir)
	if actualBase != expectedDir {
		t.Errorf("Expected output directory to be %q, got %q", expectedDir, actualBase)
	}
}

func TestExportPathConstruction(t *testing.T) {
	const projectRoot = "/test/project"
	testCases := []struct {
		platform struct{ os, arch string }
		expected string
	}{
		{
			platform: struct{ os, arch string }{"linux", "amd64"},
			expected: filepath.Join(projectRoot, "build/ai-pdf-renamer-linux-amd64"),
		},
		{
			platform: struct{ os, arch string }{"darwin", "arm64"},
			expected: filepath.Join(projectRoot, "build/ai-pdf-renamer-darwin-arm64"),
		},
		{
			platform: struct{ os, arch string }{"windows", "amd64"},
			expected: filepath.Join(projectRoot, "build/ai-pdf-renamer-windows-amd64.exe"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.platform.os+"/"+tc.platform.arch, func(t *testing.T) {
			got := getExportPath(projectRoot, tc.platform)
			if got != tc.expected {
				t.Errorf("getExportPath(%q, %+v) = %q, want %q",
					projectRoot, tc.platform, got, tc.expected)
			}
		})
	}
}
