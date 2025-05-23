# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added vision mode as the default processing mode
- Added `-novision` flag for OCR-only processing
- Added automatic model switching to qwen2.5vl:7b when vision mode is enabled
- Added comprehensive test suite for vision mode and model switching
- Added fast mode for image-based processing using qwen2.5vl:7b model
- Added PDF to image conversion using vips
- Added support for vision-language model (qwen2.5vl:7b)
- Added hardware requirements documentation for supported models
- Added detailed model selection documentation with recommendations for gemma3:1b and llama3.3:latest
- Added testing documentation to README (Go tests only, shell script testing TODO)
- Added improved command line argument handling in shell script
- Added better error messages for invalid command line options
- Added test suite for Dagger build process
- Added tests for build platforms and output paths
- Added focused test suite for configuration handling
- Added tests for default config values and flag parsing
- Added proper exit codes for Dagger build process to support CI/CD pipelines
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
- Changed default processing mode to vision-based analysis
- Changed OCR mode to be available only via `-novision` flag or as fallback
- Updated default model to qwen2.5vl:7b for vision-based processing
- Updated help text and documentation to reflect new default behavior
- Updated model hardware requirements to provide general guidance instead of specific RAM numbers
- Updated testing documentation to clarify shell script testing status
- Refactored shell script to improve function organization and maintainability
- Changed build output directory from 'bin' to 'build' for better convention
- Changed default model from llama3.3:latest to gemma3:1b for better performance and smaller size
- Updated README to include model selection documentation
- Updated README to reflect both shell script and Go implementations
- Enhanced prompt handling for better filename generation
- Improved error messages and user feedback
- Improved build documentation with Dagger advantages
- Standardized command examples to use generic binary name

### Fixed
- Fixed model switching logic to ensure correct model is used in vision mode
- Fixed flag handling for `-novision` to properly disable vision processing
- Potential risk of file operations when required model is not installed
- Various shell script compatibility issues
- Error handling for missing dependencies
- File permission handling

### Removed
- Removed shell script implementation in favor of Go version
- Removed shell script testing documentation
- Removed shell script examples from documentation
- Docker-related components and documentation
- Dockerfile and .dockerignore files
