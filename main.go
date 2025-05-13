package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultPrompt = "Extract the most important keywords from this text and create a filename. The filename should be concise (max 64 chars), use only the most important keywords, and separate words with dashes. Do not include any explanations or additional text."

// Config holds the application configuration
type Config struct {
	AutoRename   bool
	CustomPrompt string
	Model        string
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
	deps := []string{"ocrmypdf", "curl", "jq", "ollama"}
	for _, dep := range deps {
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

// getDefaultConfig returns the default configuration
func getDefaultConfig() Config {
	return Config{
		AutoRename:   false,
		CustomPrompt: defaultPrompt,
		Model:        "gemma3:1b",
	}
}

func main() {
	// Initialize config with defaults
	defaultConfig := getDefaultConfig()
	autoRename := flag.Bool("auto", defaultConfig.AutoRename, "Automatically rename all files without confirmation")
	customPrompt := flag.String("prompt", defaultConfig.CustomPrompt, "Custom prompt for filename generation")
	model := flag.String("model", defaultConfig.Model, "Ollama model to use for filename generation")
	flag.Parse()

	// Initialize config
	config = Config{
		AutoRename:   *autoRename,
		CustomPrompt: *customPrompt,
		Model:        *model,
	}

	// Check dependencies
	if err := checkDependencies(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Get file patterns from arguments
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: ai-pdf-renamer [OPTIONS] [FILE_PATTERNS...]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  ai-pdf-renamer '*.pdf'                    # Process all PDF files")
		fmt.Println("  ai-pdf-renamer '*infographic*.pdf'        # Process files containing 'infographic'")
		fmt.Println("  ai-pdf-renamer file1.pdf file2.pdf        # Process specific files")
		fmt.Println("  cat filelist.txt | xargs ai-pdf-renamer   # Process files listed in filelist.txt")
		fmt.Println("  ai-pdf-renamer -p 'custom prompt' *.pdf   # Use custom prompt for filename generation")
		os.Exit(1)
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

			fmt.Printf("Processing: %s\n", pdfFile)

			// Extract text using OCR
			text, err := extractText(pdfFile)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			fmt.Printf("Extracted text length: %d characters\n", len(text))

			// Generate new filename
			prompt := config.CustomPrompt + " Text: " + text
			newName, err := generateFilename(text, prompt)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// If auto_rename is set, rename automatically
			if *autoRename {
				newPath := newName + ".pdf"
				if err := os.Rename(pdfFile, newPath); err != nil {
					fmt.Printf("Error renaming file: %v\n", err)
					continue
				}
				fmt.Printf("File automatically renamed to: %s\n", newPath)
				continue
			}

			// Ask for confirmation
			fmt.Printf("Suggested new filename: %s.pdf\n", newName)
			fmt.Println("Options:")
			fmt.Println("  y - Rename file")
			fmt.Println("  n - Keep original name")
			fmt.Println("  a - Rename all remaining files automatically")
			fmt.Print("Choose an option (y/n/a): ")

			var confirm string
			fmt.Scanln(&confirm)

			switch strings.ToLower(confirm) {
			case "y", "yes":
				newPath := newName + ".pdf"
				if err := os.Rename(pdfFile, newPath); err != nil {
					fmt.Printf("Error renaming file: %v\n", err)
					continue
				}
				fmt.Println("File renamed successfully.")
			case "a":
				newPath := newName + ".pdf"
				if err := os.Rename(pdfFile, newPath); err != nil {
					fmt.Printf("Error renaming file: %v\n", err)
					continue
				}
				fmt.Println("File renamed successfully.")
				*autoRename = true
			default:
				fmt.Println("File kept with original name.")
			}
		}
	}

	fmt.Println("Processing complete!")
}
