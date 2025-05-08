![image](https://github.com/user-attachments/assets/8512bc6d-9522-41b8-ac68-679a931aec7a)


# AI PDF Renamer

**AI PDF Renamer** is a simple but powerful script that automatically renames PDF files based on their content using AI. By leveraging the capabilities of OpenAI's GPT models, the script reads the text from PDFs and intelligently suggests a more descriptive and context-aware filename. This helps users keep their file libraries organized without the need to open each file and rename it manually.

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

3. Make the script executable:
   ```bash
   chmod +x process_pdfs.sh
   ```

## Usage

```bash
./process_pdfs.sh [OPTIONS] [FILE_PATTERNS...]
```

### Options

- `-h, --help`: Show help message
- `-a, --auto`: Automatically rename all files without confirmation
- `-p, --prompt`: Use a custom prompt for filename generation

### Examples

1. Process all PDF files in current directory:
   ```bash
   ./process_pdfs.sh '*.pdf'
   ```

2. Process specific files:
   ```bash
   ./process_pdfs.sh file1.pdf file2.pdf
   ```

3. Process files with custom prompt:
   ```bash
   ./process_pdfs.sh -p "Create a filename that emphasizes the main topic and date from this text: $text" '*.pdf'
   ```

4. Process files automatically without confirmation:
   ```bash
   ./process_pdfs.sh -a '*.pdf'
   ```

5. Process files from a list:
   ```bash
   cat filelist.txt | xargs ./process_pdfs.sh
   ```

## How it Works

1. The script processes each PDF file using OCR to extract text content
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

## Notes

- The script requires Ollama to be running locally on port 11434
- Generated filenames are limited to 64 characters
- Only alphanumeric characters and dashes are allowed in generated filenames
- The script will skip non-PDF files and non-existent files
