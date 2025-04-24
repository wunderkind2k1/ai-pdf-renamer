# PDF Processing Script

This script processes PDF files containing "infographic" in their filename, performs OCR if needed, and generates descriptive filenames using Ollama's AI capabilities.

## Features

- Automatically processes all PDF files containing "infographic" in their filename
- Performs OCR on PDFs using `ocrmypdf`
- Extracts text content from PDFs
- Generates concise, descriptive filenames using Ollama's AI
- Interactive renaming with options for single or batch processing

## Requirements

- `ocrmypdf` - For OCR processing
- `curl` - For API calls
- `jq` - For JSON processing
- Ollama running locally with the `llama3.3:latest` model

## Installation

1. Ensure you have the required dependencies installed:
   ```bash
   # Install ocrmypdf
   pip install ocrmypdf

   # Install jq (if not already installed)
   # On macOS:
   brew install jq
   # On Ubuntu/Debian:
   sudo apt-get install jq
   ```

2. Make sure Ollama is running locally with the llama3.3:latest model:
   ```bash
   ollama run llama3.3:latest
   ```

## Usage

1. Make the script executable:
   ```bash
   chmod +x process_pdfs.sh
   ```

2. Run the script:
   ```bash
   ./process_pdfs.sh
   ```

3. For each PDF file, you'll be presented with options:
   - `y` - Rename the current file
   - `n` - Keep the original name
   - `a` - Rename all remaining files automatically

## How it Works

1. The script scans the current directory for PDF files containing "infographic" in their name
2. For each file:
   - Performs OCR if needed
   - Extracts text content
   - Sends the text to Ollama to generate a descriptive filename
   - Presents renaming options
3. Generated filenames are:
   - Concise (max 64 characters)
   - Contain only important keywords
   - Use dashes to separate words
   - Include only alphanumeric characters and dashes

## Notes

- The script modifies files in place
- Original files are replaced with OCR'd versions
- Generated filenames are based on the content of the PDFs
- The script might require an active internet connection for Ollama API calls
