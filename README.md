![image](https://github.com/user-attachments/assets/8512bc6d-9522-41b8-ac68-679a931aec7a)


# AI PDF Renamer

**AI PDF Renamer** is a simple but powerful tool that automatically renames PDF files based on their content using AI. By leveraging the capabilities of Ollama's AI models, the tool reads the text from PDFs and intelligently suggests a more descriptive and context-aware filename. This helps users keep their file libraries organized without the need to open each file and rename it manually.

### 🔍 Why Use It?

Manually renaming downloaded or scanned PDFs — like research papers, invoices, e-books, or contracts — is tedious and time-consuming. Often, files are saved with generic names such as `document.pdf`, `file(3).pdf`, or `scan_2024_05_01.pdf`. This tool solves that problem by reading the document and renaming it to something meaningful based on its contents.

### 💡 Use Cases

- **Researchers & Academics**: Automatically rename papers downloaded from journals to include the title and authors.
- **Students**: Keep your coursework, notes, and study materials neatly labeled and searchable.
- **Professionals**: Organize invoices, contracts, and reports without having to open and scan each document manually.
- **Anyone with a messy Downloads folder**: Bring order to chaos by turning vague file names into descriptive ones.

## Features

- Automatically processes all PDF files containing for example "infographic" in their filename
- Performs OCR on PDFs using `ocrmypdf`
- Extracts text content from PDFs
- Generates concise, descriptive filenames using Ollama's AI
- Interactive renaming with options for single or batch processing
- Available in both shell script and Go implementations
- Cross-platform support (Linux, macOS, Windows)

## Requirements

- `ocrmypdf`: For PDF text extraction
- `curl`: For making API requests
- `jq`: For JSON processing
- `Ollama`: Running locally with the llama3.3 model

## Installation

1. Make sure you have the required dependencies installed:
   ```bash
   # macOS
   brew install ocrmypdf curl jq
   brew install ollama
   ```

2. Download and set up the llama3.3 model:
   ```bash
   # Start Ollama service
   ollama serve

   # In a new terminal, pull the llama3.3 model
   ollama pull llama3.3:latest
   ```

3. Choose your preferred implementation:

   ### Shell Script Version
   ```bash
   chmod +x process_pdfs.sh
   ```

   ### Go Version
   ```bash
   # Standard build
   go build -o ai-pdf-renamer main.go

   # Or use the Dagger build for cross-platform binaries
   cd dagger
   go run main.go
   ```

## Usage

Both implementations provide the same functionality and command-line interface:

```bash
./ai-pdf-renamer [OPTIONS] [FILE_PATTERNS...]
```

### Options

- `-h, --help`: Show help message
- `-a, --auto`: Automatically rename all files without confirmation
- `-p, --prompt`: Use a custom prompt for filename generation

### Examples

1. Process all PDF files in current directory:
   ```bash
   ./ai-pdf-renamer '*.pdf'
   ```

2. Process specific files:
   ```bash
   ./ai-pdf-renamer file1.pdf file2.pdf
   ```

3. Process files with custom prompt:
   ```bash
   ./ai-pdf-renamer -p "Create a filename that emphasizes the main topic and date from this text: $text" '*.pdf'
   ```

4. Process files automatically without confirmation:
   ```bash
   ./ai-pdf-renamer -a '*.pdf'
   ```

5. Process files from a list:
   ```bash
   cat filelist.txt | xargs ./ai-pdf-renamer
   ```

## How it Works

1. The tool processes each PDF file using OCR to extract text content
2. The extracted text is sent to Ollama's AI model (llama3.3) to generate a meaningful filename
3. For each file, you can:
   - Accept the suggested filename
   - Keep the original name
   - Automatically rename all remaining files
   - Use a custom prompt for filename generation

## Default Prompt

The default prompt used for filename generation is:
```
Extract the most important keywords from this text and create a filename. The filename should be concise (max 64 chars), use only the most important keywords, and separate words with dashes. Do not include any explanations or additional text.
```

You can override this using the `-p/--prompt` option.

## Build Options

The Go implementation can be built in two ways:

1. **Standard Go Build**
   ```bash
   go build -o ai-pdf-renamer main.go
   ```
   This creates a binary for your current platform.

2. **Cross-Platform Build with Dagger**
   ```bash
   cd dagger
   go run main.go
   ```
   This creates binaries for multiple platforms:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)

   The binaries will be placed in the `bin` directory with platform-specific names.

   ### Advantages of Dagger Build
   - **Cross-Platform Support**: Builds binaries for all major platforms in a single run
   - **Minimal Requirements**: Only requires a running Docker host - no need to install Go or any other build tools (at least if you have ./dagger/main.go built as an executable)
   - **CI/CD Ready**: Can be easily integrated into CI/CD pipelines for automated builds
   - **Reproducible**: Builds are consistent across different environments thanks to containerization
   - **Isolated**: Build process runs in containers, ensuring no conflicts with local development environment

## Notes

- The tool requires Ollama to be running locally on port 11434
- Generated filenames are limited to 64 characters
- Only alphanumeric characters and dashes are allowed in generated filenames
- The tool will skip non-PDF files and non-existent files
