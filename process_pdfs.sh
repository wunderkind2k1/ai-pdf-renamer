#!/usr/bin/env bash

# Check if required commands are available
command -v ocrmypdf >/dev/null 2>&1 || { echo "Error: ocrmypdf is not installed. Please install it first."; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "Error: curl is not installed. Please install it first."; exit 1; }

# Initialize auto_rename flag
auto_rename=false

# Function to extract text from PDF using ocrmypdf sidecar
extract_text() {
    local pdf_file="$1"
    local text_file="${pdf_file%.pdf}.txt"

    # Run OCR with sidecar text file
    if ! ocrmypdf "$pdf_file" "$pdf_file" --force-ocr --sidecar "$text_file"; then
        echo "Error: OCR failed for $pdf_file"
        return 1
    fi

    # Read the text file
    if [ -f "$text_file" ]; then
        local text=$(cat "$text_file")
        # Clean up the text file
        rm "$text_file"
        echo "$text"
    else
        echo "Error: Text file not created"
        return 1
    fi
}

# Function to generate filename using Ollama API
generate_filename() {
    local text="$1"
    local prompt="Extract the most important keywords from this text and create a filename. The filename should be concise (max 64 chars), use only the most important keywords, and separate words with dashes. Do not include any explanations or additional text. Text: $text"

    # Escape the prompt text for JSON
    local escaped_prompt=$(echo "$prompt" | jq -Rs .)

    # Create the JSON payload
    local json_payload="{\"model\":\"llama3.3:latest\",\"prompt\":$escaped_prompt,\"stream\":false}"

    # Call Ollama API
    local response=$(curl -s -X POST http://localhost:11434/api/generate \
        -H "Content-Type: application/json" \
        -d "$json_payload")

    if [ -z "$response" ]; then
        echo "Error: No response from Ollama API"
        return 1
    fi

    # Extract the response text from the JSON
    local response_text=$(echo "$response" | jq -r '.response')

    if [ -z "$response_text" ]; then
        echo "Error: Could not parse response from Ollama API"
        return 1
    fi

    # Clean up the response (remove quotes, extra spaces, etc.)
    local clean_name=$(echo "$response_text" | tr -d '"' | tr -s ' ' | tr ' ' '-' | tr -cd '[:alnum:]-')

    # Ensure the name is not too long
    if [ ${#clean_name} -gt 64 ]; then
        clean_name="${clean_name:0:64}"
    fi

    echo "$clean_name"
}

# Process each PDF file containing 'infographic' in the name
for pdf_file in *infographic*.pdf; do
    if [ ! -f "$pdf_file" ]; then
        echo "No PDF files containing 'infographic' found in the current directory."
        exit 1
    fi

    echo "Processing: $pdf_file"

    # Extract text using OCR
    text=$(extract_text "$pdf_file")

    if [ -z "$text" ]; then
        echo "Error: Could not extract text from $pdf_file"
        continue
    fi

    echo "Extracted text length: ${#text} characters"

    # Generate new filename
    new_name=$(generate_filename "$text")

    if [ -z "$new_name" ]; then
        echo "Error: Could not generate filename for $pdf_file"
        continue
    fi

    # If auto_rename is set, rename automatically
    if [ "$auto_rename" = true ]; then
        mv "$pdf_file" "${new_name}.pdf"
        echo "File automatically renamed to: ${new_name}.pdf"
        continue
    fi

    # Ask for confirmation
    echo "Suggested new filename: $new_name.pdf"
    echo "Options:"
    echo "  y - Rename file"
    echo "  n - Keep original name"
    echo "  a - Rename all remaining files automatically"
    read -p "Choose an option (y/n/a): " confirm

    if [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]]; then
        # Rename the file
        mv "$pdf_file" "${new_name}.pdf"
        echo "File renamed successfully."
    elif [[ $confirm == [aA] ]]; then
        # Rename the file
        mv "$pdf_file" "${new_name}.pdf"
        echo "File renamed successfully."
        # Set auto_rename flag for remaining files
        auto_rename=true
    else
        echo "File kept with original name."
    fi
done

echo "Processing complete!"
