package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dagger.io/dagger"
)

// BuildPlatforms defines the target platforms for cross-compilation
var BuildPlatforms = []struct {
	os   string
	arch string
}{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
}

// getOutputDir returns the absolute path to the output directory
func getOutputDir() (string, error) {
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		return "", fmt.Errorf("error getting absolute path: %w", err)
	}
	return filepath.Join(projectRoot, "build"), nil
}

// getExportPath returns the path where a binary should be exported for a given platform
func getExportPath(projectRoot string, platform struct{ os, arch string }) string {
	output := fmt.Sprintf("build/ai-pdf-renamer-%s-%s", platform.os, platform.arch)
	if platform.os == "windows" {
		output += ".exe"
	}
	return filepath.Join(projectRoot, output)
}

func main() {
	ctx := context.Background()
	failedBuilds := 0

	fmt.Println("🚀 Starting Dagger build process...")

	// Initialize Dagger client
	fmt.Println("📡 Connecting to Dagger...")
	client, err := dagger.Connect(ctx)
	if err != nil {
		fmt.Printf("❌ Error connecting to Dagger: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	fmt.Println("✅ Connected to Dagger")

	// Get absolute path to project root
	fmt.Println("📂 Setting up project paths...")
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		fmt.Printf("❌ Error getting absolute path: %v\n", err)
		os.Exit(1)
	}

	// Get source code from project root
	src := client.Host().Directory(projectRoot)

	// Create Go container
	fmt.Println("🐳 Setting up Go build environment...")
	container := client.Container().From("golang:1.24")

	// Mount source code
	container = container.WithMountedDirectory("/src", src)
	container = container.WithWorkdir("/src")

	// Create output directory if it doesn't exist
	outputDir, err := getOutputDir()
	if err != nil {
		fmt.Printf("❌ Error getting output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("❌ Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🏗️  Starting builds for %d platforms...\n", len(BuildPlatforms))

	// Build for each platform
	for i, platform := range BuildPlatforms {
		startTime := time.Now()
		fmt.Printf("\n📦 Building for %s/%s (%d/%d)...\n",
			platform.os, platform.arch, i+1, len(BuildPlatforms))

		// Set environment variables for cross-compilation
		container = container.WithEnvVariable("GOOS", platform.os)
		container = container.WithEnvVariable("GOARCH", platform.arch)
		container = container.WithEnvVariable("CGO_ENABLED", "0")

		// Build the binary
		output := fmt.Sprintf("build/ai-pdf-renamer-%s-%s", platform.os, platform.arch)
		if platform.os == "windows" {
			output += ".exe"
		}

		// Execute the build
		built := container.WithExec([]string{"go", "build", "-o", output, "main.go"})

		// Export the binary
		exportPath := getExportPath(projectRoot, platform)
		_, err := built.File(output).Export(ctx, exportPath)
		if err != nil {
			fmt.Printf("❌ Error exporting binary for %s/%s: %v\n", platform.os, platform.arch, err)
			failedBuilds++
			continue
		}

		duration := time.Since(startTime)
		fmt.Printf("✅ Successfully built and exported for %s/%s in %v\n",
			platform.os, platform.arch, duration.Round(time.Millisecond))
	}

	if failedBuilds > 0 {
		fmt.Printf("\n❌ Build process completed with %d failed builds!\n", failedBuilds)
		os.Exit(1)
	}

	fmt.Println("\n🎉 Build process completed successfully!")
}
