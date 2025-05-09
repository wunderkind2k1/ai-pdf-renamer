package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dagger.io/dagger"
)

func main() {
	ctx := context.Background()

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

	// Define platforms to build for
	platforms := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}

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

	// Create bin directory if it doesn't exist
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		fmt.Printf("❌ Error creating bin directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🏗️  Starting builds for %d platforms...\n", len(platforms))

	// Build for each platform
	for i, platform := range platforms {
		startTime := time.Now()
		fmt.Printf("\n📦 Building for %s/%s (%d/%d)...\n",
			platform.os, platform.arch, i+1, len(platforms))

		// Set environment variables for cross-compilation
		container = container.WithEnvVariable("GOOS", platform.os)
		container = container.WithEnvVariable("GOARCH", platform.arch)
		container = container.WithEnvVariable("CGO_ENABLED", "0")

		// Build the binary
		output := fmt.Sprintf("bin/ai-pdf-renamer-%s-%s", platform.os, platform.arch)
		if platform.os == "windows" {
			output += ".exe"
		}

		// Execute the build
		built := container.WithExec([]string{"go", "build", "-o", output, "main.go"})

		// Export the binary
		_, err := built.File(output).Export(ctx, filepath.Join(projectRoot, output))
		if err != nil {
			fmt.Printf("❌ Error exporting binary for %s/%s: %v\n", platform.os, platform.arch, err)
			continue
		}

		duration := time.Since(startTime)
		fmt.Printf("✅ Successfully built and exported for %s/%s in %v\n",
			platform.os, platform.arch, duration.Round(time.Millisecond))
	}

	fmt.Println("\n🎉 Build process completed!")
}
