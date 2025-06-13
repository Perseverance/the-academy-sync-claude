# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This appears to be a new Go project named `the-academy-sync-claude` that is currently in its initial setup phase. The repository contains only basic configuration files.

## Development Setup

Based on the .gitignore file, this is intended to be a Go project. Common Go development commands that will likely be needed:

- `go mod init` - Initialize a new Go module (if not already done)
- `go build` - Build the Go application
- `go run .` or `go run main.go` - Run the application directly
- `go test ./...` - Run all tests in the project
- `go test -v ./...` - Run tests with verbose output
- `go test -cover ./...` - Run tests with coverage information
- `go fmt ./...` - Format all Go source files
- `go vet ./...` - Run static analysis on the code

## Architecture

The project structure is not yet established. When source code is added, typical Go project organization should be followed with appropriate package structure.