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

// Defaults target LM Studio's OpenAI-compatible server; override via env vars
// to use any other OpenAI-style endpoint (Ollama's /v1, llama.cpp, vLLM, OpenAI, ...).
const (
	defaultAPIBaseURL = "http://localhost:1234/v1"
	defaultModel      = "google/gemma-4-e4b"

	envAPIBaseURL = "AI_PDF_RENAMER_API_URL"
	envAPIKey     = "AI_PDF_RENAMER_API_KEY"
	envModel      = "AI_PDF_RENAMER_MODEL"
)

const defaultPrompt = "Extract the most important keywords from this text and create a filename. The filename should be concise (max 64 chars), use only the most important keywords, and separate words with dashes. Do not include any explanations or additional text."
const defaultMaxChars = 64
const winErrPre = "On windows dependencies might be tricky. We recommend using a package manager like chocolatey, winget or similar\n\n"

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

// maxCharsLimit controls the truncation length for generated filenames
var maxCharsLimit = defaultMaxChars

// chatCompletionResponse represents the response from an OpenAI-style /chat/completions endpoint
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	// Error is an object ({"message": ...}) for OpenAI/LM Studio, but some servers return a plain string
	Error json.RawMessage `json:"error,omitempty"`
}

// envOr returns the first non-empty environment variable from keys, or fallback
func envOr(fallback string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return fallback
}

// apiBaseURL returns the OpenAI-style base URL (e.g. http://localhost:1234/v1) without trailing slash
func apiBaseURL() string {
	return strings.TrimRight(envOr(defaultAPIBaseURL, envAPIBaseURL, "OPENAI_BASE_URL"), "/")
}

// newAPIRequest builds a request against the configured API, adding auth if a key is set
func newAPIRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, apiBaseURL()+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := envOr("", envAPIKey, "OPENAI_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return req, nil
}

// apiErrorMessage extracts a readable message from an OpenAI-style error field
func apiErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Message != "" {
		return obj.Message
	}
	return string(raw)
}

// chatCompletion sends a single user message (text plus optional base64 PNG images) and returns the reply
func chatCompletion(prompt string, base64Images []string) (string, error) {
	var content interface{} = prompt
	if len(base64Images) > 0 {
		parts := []map[string]interface{}{{"type": "text", "text": prompt}}
		for _, img := range base64Images {
			parts = append(parts, map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]string{"url": "data:image/png;base64," + img},
			})
		}
		content = parts
	}

	payload := map[string]interface{}{
		"model":    config.Model,
		"messages": []map[string]interface{}{{"role": "user", "content": content}},
		"stream":   false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error creating JSON payload: %v", err)
	}

	req, err := newAPIRequest(http.MethodPost, "/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error calling API at %s: %v", apiBaseURL(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var chatResp chatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("error parsing response (HTTP %d): %v", resp.StatusCode, err)
	}

	if msg := apiErrorMessage(chatResp.Error); msg != "" {
		return "", fmt.Errorf("error from API: %s\nPlease ensure that the %s model is available (e.g. in LM Studio: lms get %s)", msg, config.Model, config.Model)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from API: HTTP %d: %s", resp.StatusCode, string(body))
	}

	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("error: Empty response from API\nPlease ensure that the %s model is available and working correctly", config.Model)
	}

	return chatResp.Choices[0].Message.Content, nil
}

// cleanFilename converts a model reply into a safe, length-limited filename
func cleanFilename(response string) string {
	cleanName := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(response, "-")
	cleanName = regexp.MustCompile(`-+`).ReplaceAllString(cleanName, "-")
	cleanName = strings.Trim(cleanName, "-")

	// Ensure the name is not too long
	if len(cleanName) > maxCharsLimit {
		cleanName = cleanName[:maxCharsLimit]
	}

	return cleanName
}

// checkDependencies verifies that all required tools are installed
func checkDependencies() error {
	deps := []string{"curl", "jq", "gs", "ocrmypdf"} // Always include ocrmypdf
	for _, dep := range deps {
		if runtime.GOOS == "windows" {
			// On Windows, append .exe if necessary
			dep = dep + ".exe"
		}

		if _, err := exec.LookPath(dep); err != nil {
			depErrMsg := "error: %s is not installed. Please install it first"
			if runtime.GOOS == "windows" {
				depErrMsg = winErrPre + depErrMsg
			}
			return fmt.Errorf(depErrMsg, dep)
		}
	}

	// Check if the API server is reachable and the specified model is available
	req, err := newAPIRequest(http.MethodGet, "/models", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error: API server at %s is not reachable.\nStart the LM Studio server (lms server start) or set %s to an OpenAI-compatible endpoint", apiBaseURL(), envAPIBaseURL)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading models response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error listing models at %s: HTTP %d: %s", apiBaseURL(), resp.StatusCode, string(body))
	}

	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		return fmt.Errorf("error parsing models response: %v", err)
	}

	var available []string
	for _, model := range models.Data {
		if model.ID == config.Model {
			return nil
		}
		available = append(available, model.ID)
	}

	return fmt.Errorf("error: %s model is not available at %s.\nIn LM Studio, download it with: lms get %s\nor pick one via -model or %s. Available models: %s", config.Model, apiBaseURL(), config.Model, envModel, strings.Join(available, ", "))
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

// generateFilename generates a filename using the OpenAI-compatible chat API
func generateFilename(text string, prompt string) (string, error) {
	response, err := chatCompletion(prompt, nil)
	if err != nil {
		return "", err
	}
	return cleanFilename(response), nil
}

// generateFilenameFast generates a filename using the OpenAI-compatible chat API with multiple image inputs
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

	response, err := chatCompletion(prompt, base64Images)
	if err != nil {
		return "", err
	}
	return cleanFilename(response), nil
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() Config {
	return Config{
		AutoRename:   false,
		CustomPrompt: defaultPrompt,
		Model:        envOr(defaultModel, envModel), // Default to vision model
		FastMode:     true,                          // Default to vision mode
		OutputDir:    "",                            // Empty string means use the same directory as input
		Exitor:       &DefaultExitor{},              // Default exitor implementation
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
	model := flag.String("model", defaultConfig.Model, "Model to use for filename generation (env: "+envModel+")")
	noVision := flag.Bool("novision", false, "Disable vision-based processing and use OCR only")
	outputDir := flag.String("output", "", "Output directory for renamed files (default: same as input)")
	maxChars := flag.Int("maxchars", defaultMaxChars, "Maximum character length for generated filenames")

	// Custom usage function to provide clearer help
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -output renamed/ *.pdf     # Use vision-based processing (default)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -novision *.pdf            # Use OCR-only mode\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -auto *.pdf                # Process all PDFs automatically\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -model qwen/qwen2.5-vl-7b *.pdf # Use a different model\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -maxchars 64 *.pdf           # Limit filenames to 64 characters\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  %s  OpenAI-compatible base URL (default: %s, fallback: OPENAI_BASE_URL)\n", envAPIBaseURL, defaultAPIBaseURL)
		fmt.Fprintf(os.Stderr, "  %s  API key sent as Bearer token (optional, fallback: OPENAI_API_KEY)\n", envAPIKey)
		fmt.Fprintf(os.Stderr, "  %s    Default model (default: %s)\n", envModel, defaultModel)
		fmt.Fprintf(os.Stderr, "\nNote: Vision-based processing is enabled by default. Use -novision to disable it and use OCR only.\n")
	}

	flag.Parse()

	// Set the global maxchars limit for filename truncation
	maxCharsLimit = *maxChars

	// If maxchars differs from default, inform the AI about the desired length
	prompt := *customPrompt
	if *maxChars != defaultMaxChars {
		prompt += fmt.Sprintf(" The filename will be cut off at %d characters.", *maxChars)
	}

	// Build config from flags
	cfg := Config{
		AutoRename:   *autoRename,
		CustomPrompt: prompt,
		Model:        *model,
		FastMode:     !*noVision, // Invert the novision flag to get FastMode
		OutputDir:    *outputDir,
		Exitor:       &DefaultExitor{},
	}

	setup(cfg)
}
