# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added model selection option (`-m` or `--model`) to both shell script and Go implementations
- Added explicit model availability check at startup for both shell script and Go implementations
- Added improved error handling for model-related issues
- Added prevention of file operations when required model is not available
- Initial shell script implementation
- PDF text extraction using ocrmypdf
- AI-powered filename generation using Ollama
- Interactive renaming with confirmation
- Support for custom prompts
- Batch processing capability
- Error handling and dependency checks
- Basic documentation and usage examples
- Go implementation of the PDF renamer
- Cross-platform build support using Dagger
- Multi-platform binary generation (Linux, macOS, Windows)
- Progress indicators for build process
- Automated binary export from build containers

### Changed
- Changed default model from llama3.3:latest to gemma:1b for better performance and smaller size
- Updated README to include model selection documentation
- Improved error messages and user feedback
- Enhanced prompt handling for better filename generation
- Updated README to reflect both shell script and Go implementations
- Improved build documentation with Dagger advantages
- Standardized command examples to use generic binary name

### Fixed
- Potential risk of file operations when required model is not installed
- Various shell script compatibility issues
- Error handling for missing dependencies
- File permission handling

### Removed
- Docker-related components and documentation
- Dockerfile and .dockerignore files
