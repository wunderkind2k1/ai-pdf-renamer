package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Exitor defines the interface for program exit behavior
type Exitor interface {
	Exit(code int)
}

// DefaultExitor implements Exitor using os.Exit
type DefaultExitor struct{}

func (e *DefaultExitor) Exit(code int) {
	os.Exit(code)
}

const defaultPrompt = "Extract the most important keywords from this text and create a filename. The filename should be concise (max 64 chars), use only the most important keywords, and separate words with dashes. Do not include any explanations or additional text."

// Config holds the application configuration
type Config struct {
	AutoRename   bool
	CustomPrompt string
	Model        string
	FastMode     bool
	OutputDir    string // New field for output directory
	Exitor       Exitor // Interface for program exit behavior
}

// Global config variable
var config Config

// OllamaResponse represents the response from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// checkDependencies verifies that all required tools are installed
func checkDependencies() error {
	deps := []string{"curl", "jq", "ollama", "gs", "ocrmypdf"} // Always include ocrmypdf
	for _, dep := range deps {
		if runtime.GOOS == "windows" {
			// On Windows, append .exe if necessary
			dep = dep + ".exe"
		}

		if _, err := exec.LookPath(dep); err != nil {
			return fmt.Errorf("error: %s is not installed. Please install it first", dep)
		}
	}

	// Check if Ollama service is running
	resp, err := http.Get("http://localhost:11434/api/version")
	if err != nil {
		return fmt.Errorf("error: Ollama service is not running. Please start it with 'ollama serve'")
	}
	defer resp.Body.Close()

	// Check if the specified model is available
	resp, err = http.Get("http://localhost:11434/api/tags")
	if err != nil {
		return fmt.Errorf("error checking Ollama models: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading Ollama models response: %v", err)
	}

	var models struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		return fmt.Errorf("error parsing Ollama models response: %v", err)
	}

	modelFound := false
	for _, model := range models.Models {
		if model.Name == config.Model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		return fmt.Errorf("error: %s model is not installed in Ollama.\nPlease install it by running: ollama pull %s", config.Model, config.Model)
	}

	return nil
}

// extractText extracts text from a PDF using ocrmypdf
func extractText(pdfFile string) (string, error) {
	textFile := strings.TrimSuffix(pdfFile, ".pdf") + ".txt"

	// Run OCR with sidecar text file
	cmd := exec.Command("ocrmypdf", pdfFile, pdfFile,
		"--force-ocr",
		"--sidecar", textFile,
		"--optimize", "0",
		"--output-type", "pdf",
		"--fast-web-view", "0")

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error: OCR failed for %s: %v", pdfFile, err)
	}

	// Read the text file
	content, err := os.ReadFile(textFile)
	if err != nil {
		return "", fmt.Errorf("error: Text file not created: %v", err)
	}

	// Clean up the text file
	os.Remove(textFile)

	return string(content), nil
}

// validatePNG checks if the provided byte slice is a valid PNG image
func validatePNG(data []byte) error {
	_, err := png.DecodeConfig(bytes.NewReader(data))
	return err
}

// extractPageAsPNG extracts a single page from a PDF as a PNG image using Ghostscript, in-memory
func extractPageAsPNG(pdfPath string, page int) ([]byte, error) {
	cmd := exec.Command(
		"gs",
		"-q",              // Quiet mode (no output)
		"-dNOPAUSE",       // No pause after page
		"-sDEVICE=png16m", // PNG format (24-bit color)
		"-r300",           // 300 DPI resolution
		"-dFirstPage="+fmt.Sprintf("%d", page),
		"-dLastPage="+fmt.Sprintf("%d", page),
		"-sOutputFile=-", // Output to stdout
		pdfPath,
	)

	// Create a pipe for stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %v", err)
	}

	// Capture stderr for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting Ghostscript: %v", err)
	}

	// Pre-allocate a large buffer for PNG data (e.g., 10MB)
	var out bytes.Buffer
	out.Grow(10 * 1024 * 1024)

	// Copy stdout to buffer
	if _, err := io.Copy(&out, stdout); err != nil {
		return nil, fmt.Errorf("error reading stdout: %v", err)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Ghostscript error: %v, stderr: %s", err, stderr.String())
	}

	// Get the PNG data
	pngData := out.Bytes()
	if len(pngData) == 0 {
		return nil, fmt.Errorf("no PNG data produced, stderr: %s", stderr.String())
	}

	// Validate the PNG data
	if err := validatePNG(pngData); err != nil {
		return nil, fmt.Errorf("invalid PNG data: %v, stderr: %s", err, stderr.String())
	}

	return pngData, nil
}

// extractPDFPages extracts up to 3 pages from a PDF as PNG images
func extractPDFPages(pdfFile string) ([][]byte, error) {
	var images [][]byte
	maxPages := 3

	for page := 1; page <= maxPages; page++ {
		imgData, err := extractPageAsPNG(pdfFile, page)
		if err != nil {
			// If we can't extract a page, assume we've reached the end
			break
		}
		images = append(images, imgData)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("error: could not extract any pages from PDF")
	}

	return images, nil
}

// generateFilename generates a filename using Ollama API
func generateFilename(text string, prompt string) (string, error) {
	// Create the JSON payload
	payload := map[string]interface{}{
		"model":  config.Model,
		"prompt": prompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating JSON payload: %v", err)
	}

	// Call Ollama API
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error calling Ollama API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("error from Ollama API: %s\nPlease ensure that the %s model is installed by running:\n  ollama pull %s", ollamaResp.Error, config.Model, config.Model)
	}

	if ollamaResp.Response == "" {
		return "", fmt.Errorf("error: Empty response from Ollama API\nPlease ensure that the %s model is installed and working correctly:\n  1. Check if the model is installed: ollama list\n  2. If not installed, run: ollama pull %s\n  3. If installed but not working, try: ollama rm %s && ollama pull %s", config.Model, config.Model, config.Model, config.Model)
	}

	// Clean up the response
	cleanName := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(ollamaResp.Response, "-")
	cleanName = regexp.MustCompile(`-+`).ReplaceAllString(cleanName, "-")
	cleanName = strings.Trim(cleanName, "-")

	// Ensure the name is not too long
	if len(cleanName) > 64 {
		cleanName = cleanName[:64]
	}

	return cleanName, nil
}

// generateFilenameFast generates a filename using Ollama API with multiple image inputs
func generateFilenameFast(images [][]byte, prompt string) (string, error) {
	fmt.Printf("Using model: %s for image-based processing\n", config.Model)
	fmt.Printf("Extracted %d page(s) from PDF, sending all for analysis\n", len(images))

	if len(images) == 0 {
		return "", fmt.Errorf("no images extracted from PDF")
	}

	var base64Images []string
	for i, imgData := range images {
		fmt.Printf("Page %d: Image size: %d bytes\n", i+1, len(imgData))
		if len(imgData) > 8 && string(imgData[:8]) == "\x89PNG\r\n\x1a\n" {
			fmt.Printf("Page %d: Valid PNG signature detected\n", i+1)
		} else {
			fmt.Printf("Page %d: Warning - Image data does not appear to be a valid PNG\n", i+1)
		}
		base64Images = append(base64Images, base64.StdEncoding.EncodeToString(imgData))
	}

	// Create the JSON payload with all images
	payload := map[string]interface{}{
		"model":  config.Model,
		"prompt": prompt,
		"stream": false,
		"images": base64Images,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating JSON payload: %v", err)
	}

	// Call Ollama API
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error calling Ollama API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("error from Ollama API: %s", ollamaResp.Error)
	}

	// Clean up the response
	cleanName := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(ollamaResp.Response, "-")
	cleanName = regexp.MustCompile(`-+`).ReplaceAllString(cleanName, "-")
	cleanName = strings.Trim(cleanName, "-")

	// Ensure the name is not too long
	if len(cleanName) > 64 {
		cleanName = cleanName[:64]
	}

	return cleanName, nil
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() Config {
	return Config{
		AutoRename:   false,
		CustomPrompt: defaultPrompt,
		Model:        "qwen2.5vl:7b",   // Default to vision model
		FastMode:     true,             // Default to vision mode
		OutputDir:    "",               // Empty string means use the same directory as input
		Exitor:       &DefaultExitor{}, // Default exitor implementation
	}
}

func isImageEmpty(imgData []byte) bool {
	// Check if the image is mostly black/empty
	// We'll do this by checking if the average pixel value is very low
	// This is a simple heuristic - we could make it more sophisticated if needed
	img, err := jpeg.Decode(bytes.NewReader(imgData))
	if err != nil {
		// If we can't decode the image, assume it's not empty
		return false
	}

	bounds := img.Bounds()
	totalPixels := bounds.Dx() * bounds.Dy()
	if totalPixels == 0 {
		return true
	}

	var sumBrightness float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to grayscale using standard coefficients
			brightness := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114
			sumBrightness += brightness
		}
	}

	avgBrightness := sumBrightness / float64(totalPixels)
	// If average brightness is very low (close to black), consider it empty
	return avgBrightness < 1000 // This threshold might need adjustment
}

// writeOutputFile copies srcPath to the output directory with the given newName, returns the output path
func writeOutputFile(srcPath, newName string) (string, error) {
	outputName := newName + ".pdf"
	outputPath := outputName
	if config.OutputDir != "" {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return "", fmt.Errorf("error creating output directory: %v", err)
		}
		outputPath = filepath.Join(config.OutputDir, filepath.Base(outputName))
	}
	// Read the source file
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("error reading source file: %v", err)
	}
	// Write to the new location
	if err := os.WriteFile(outputPath, srcData, 0644); err != nil {
		return "", fmt.Errorf("error writing file: %v", err)
	}
	fmt.Printf("Renamed (saved) file to: %s\n", outputPath)
	return outputPath, nil
}

// fallbackToOCR is a helper that (if fast mode fails) falls back to OCR mode (using ocrmypdf) to extract text, generate a filename, and (if confirmed) write the output file. It returns an error if any.
func fallbackToOCR(pdfFile string) (err error) {
	fmt.Println("Falling back to OCR mode (using ocrmypdf)…")
	text, err := extractText(pdfFile)
	if err != nil {
		fmt.Printf("Error in OCR fallback (extractText): %v\n", err)
		return err
	}
	fmt.Printf("Extracted text length (OCR fallback): %d characters\n", len(text))
	prompt := config.CustomPrompt + " Text: " + text
	newName, err := generateFilename(text, prompt)
	if err != nil {
		fmt.Printf("Error in OCR fallback (generateFilename): %v\n", err)
		return err
	}
	if !config.AutoRename {
		fmt.Printf("Suggested new filename (OCR fallback): %s.pdf\n", newName)
		fmt.Println("Options:")
		fmt.Println("  y – Rename file")
		fmt.Println("  n – Keep original name")
		fmt.Println("  a – Rename all remaining files automatically")
		var confirm string
		fmt.Scanf("%s", &confirm)
		if confirm == "a" {
			config.AutoRename = true
		} else if confirm != "y" {
			fmt.Println("File kept with original name (OCR fallback).")
			return nil
		}
	}
	_, err = writeOutputFile(pdfFile, newName)
	return err
}

func processPDF(pdfFile string) error {
	fmt.Printf("Processing: %s\n", pdfFile)

	if config.FastMode {
		// Try vision-based processing first
		images, err := extractPDFPages(pdfFile)
		if err != nil {
			fmt.Printf("Error (vision mode) extracting PDF pages: %v\n", err)
			return fallbackToOCR(pdfFile)
		}
		// Use image-based processing (generateFilenameFast) with all extracted pages
		prompt := config.CustomPrompt + " Analyze these images and create a filename based on their content."
		newName, err := generateFilenameFast(images, prompt)
		if err != nil {
			fmt.Printf("Error (vision mode) generating filename (generateFilenameFast): %v\n", err)
			return fallbackToOCR(pdfFile)
		}
		if !config.AutoRename {
			fmt.Printf("Suggested new filename (vision mode): %s.pdf\n", newName)
			fmt.Println("Options:")
			fmt.Println("  y – Rename file")
			fmt.Println("  n – Keep original name")
			fmt.Println("  a – Rename all remaining files automatically")
			var confirm string
			fmt.Scanf("%s", &confirm)
			if confirm == "a" {
				config.AutoRename = true
			} else if confirm != "y" {
				fmt.Println("File kept with original name (vision mode).")
				return nil
			}
		}
		_, err = writeOutputFile(pdfFile, newName)
		return err
	} else {
		// OCR-only mode
		text, err := extractText(pdfFile)
		if err != nil {
			fmt.Printf("Error (OCR mode) extractText: %v\n", err)
			return err
		}
		fmt.Printf("Extracted text length (OCR mode): %d characters\n", len(text))
		prompt := config.CustomPrompt + " Text: " + text
		newName, err := generateFilename(text, prompt)
		if err != nil {
			fmt.Printf("Error (OCR mode) generateFilename: %v\n", err)
			return err
		}
		if !config.AutoRename {
			fmt.Printf("Suggested new filename (OCR mode): %s.pdf\n", newName)
			fmt.Println("Options:")
			fmt.Println("  y – Rename file")
			fmt.Println("  n – Keep original name")
			fmt.Println("  a – Rename all remaining files automatically")
			var confirm string
			fmt.Scanf("%s", &confirm)
			if confirm == "a" {
				config.AutoRename = true
			} else if confirm != "y" {
				fmt.Println("File kept with original name (OCR mode).")
				return nil
			}
		}
		_, err = writeOutputFile(pdfFile, newName)
		return err
	}
}

func setup(cfg Config) {
	// Check for common flag usage errors
	args := flag.Args()
	for _, arg := range args {
		if strings.HasPrefix(arg, "-novision=") {
			fmt.Fprintf(os.Stderr, "Error: -novision is a switch flag and doesn't take a value. Use just -novision instead of -novision=%s\n", strings.TrimPrefix(arg, "-novision="))
			cfg.Exitor.Exit(1)
		}
	}

	// Clean up output directory path and ensure it's set
	outputDirPath := cfg.OutputDir
	if outputDirPath != "" {
		// Clean and normalize the path
		outputDirPath = filepath.Clean(outputDirPath)
		// Remove trailing slash if present
		outputDirPath = strings.TrimSuffix(outputDirPath, string(os.PathSeparator))
		cfg.OutputDir = outputDirPath
	}

	// If vision mode is enabled (default), ensure we're using the vision model
	if cfg.FastMode && cfg.Model != "qwen2.5vl:7b" {
		fmt.Printf("Note: Switching to qwen2.5vl:7b model for vision-based processing\n")
		cfg.Model = "qwen2.5vl:7b"
	}

	// Set global config for downstream functions
	config = cfg

	// Check dependencies
	if err := checkDependencies(); err != nil {
		fmt.Println(err)
		cfg.Exitor.Exit(1)
	}

	// Get file patterns from arguments
	args = flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: ai-pdf-renamer [OPTIONS] [FILE_PATTERNS...]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  ai-pdf-renamer '*.pdf'                    # Process all PDF files")
		fmt.Println("  ai-pdf-renamer '*infographic*.pdf'        # Process files containing 'infographic'")
		fmt.Println("  ai-pdf-renamer file1.pdf file2.pdf        # Process specific files")
		fmt.Println("  ai-pdf-renamer -output renamed/ *.pdf     # Save renamed files to 'renamed' directory")
		fmt.Println("  cat filelist.txt | xargs ai-pdf-renamer   # Process files listed in filelist.txt")
		fmt.Println("  ai-pdf-renamer -p 'custom prompt' *.pdf   # Use custom prompt for filename generation")
		cfg.Exitor.Exit(1)
	}

	// Process each file pattern
	for _, pattern := range args {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Error processing pattern %s: %v\n", pattern, err)
			continue
		}

		for _, pdfFile := range matches {
			// Skip if not a PDF file
			if !strings.HasSuffix(strings.ToLower(pdfFile), ".pdf") {
				fmt.Printf("Skipping non-PDF file: %s\n", pdfFile)
				continue
			}

			if err := processPDF(pdfFile); err != nil {
				fmt.Printf("Error processing %s: %v\n", pdfFile, err)
				continue
			}
		}
	}

	fmt.Println("Processing complete!")
}

func main() {
	// Initialize config with defaults
	defaultConfig := getDefaultConfig()
	autoRename := flag.Bool("auto", defaultConfig.AutoRename, "Automatically rename all files without confirmation")
	customPrompt := flag.String("prompt", defaultConfig.CustomPrompt, "Custom prompt for filename generation")
	model := flag.String("model", defaultConfig.Model, "Ollama model to use for filename generation")
	noVision := flag.Bool("novision", false, "Disable vision-based processing and use OCR only")
	outputDir := flag.String("output", "", "Output directory for renamed files (default: same as input)")

	// Custom usage function to provide clearer help
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -output renamed/ *.pdf     # Use vision-based processing (default)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -novision *.pdf            # Use OCR-only mode\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -auto *.pdf                # Process all PDFs automatically\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -model llama3.3:latest *.pdf # Use a different model\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nNote: Vision-based processing is enabled by default. Use -novision to disable it and use OCR only.\n")
	}

	flag.Parse()

	// Build config from flags
	cfg := Config{
		AutoRename:   *autoRename,
		CustomPrompt: *customPrompt,
		Model:        *model,
		FastMode:     !*noVision, // Invert the novision flag to get FastMode
		OutputDir:    *outputDir,
		Exitor:       &DefaultExitor{},
	}

	setup(cfg)
}
