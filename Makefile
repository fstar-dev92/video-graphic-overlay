# Video Graphic Overlay GStreamer Makefile

# Variables
BINARY_NAME=video-overlay
MAIN_FILE=main.go
BUILD_DIR=build
CONFIG_FILE=config.yaml

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.version=$(shell git describe --tags --always --dirty)"
BUILD_FLAGS=-v $(LDFLAGS)

.PHONY: all build clean test deps run install help

# Default target
all: deps build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_FILE)

# Install system dependencies (Ubuntu/Debian)
install-deps-ubuntu:
	@echo "Installing GStreamer dependencies for Ubuntu/Debian..."
	sudo apt-get update
	sudo apt-get install -y \
		libgstreamer1.0-dev \
		libgstreamer-plugins-base1.0-dev \
		libgstreamer-plugins-good1.0-dev \
		libgstreamer-plugins-bad1.0-dev \
		gstreamer1.0-plugins-good \
		gstreamer1.0-plugins-bad \
		gstreamer1.0-plugins-ugly \
		gstreamer1.0-libav

# Install system dependencies (CentOS/RHEL/Fedora)
install-deps-centos:
	@echo "Installing GStreamer dependencies for CentOS/RHEL/Fedora..."
	sudo yum install -y \
		gstreamer1-devel \
		gstreamer1-plugins-base-devel \
		gstreamer1-plugins-good \
		gstreamer1-plugins-bad-free \
		gstreamer1-plugins-ugly-free \
		gstreamer1-libav

# Install system dependencies (macOS)
install-deps-macos:
	@echo "Installing GStreamer dependencies for macOS..."
	brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly gst-libav

# Create example configuration
config:
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "Creating example configuration..."; \
		cp examples/basic-text-overlay.yaml $(CONFIG_FILE); \
		echo "Configuration created: $(CONFIG_FILE)"; \
		echo "Please edit the configuration file before running the application."; \
	else \
		echo "Configuration file already exists: $(CONFIG_FILE)"; \
	fi

# Run with example configurations
run-basic:
	./$(BUILD_DIR)/$(BINARY_NAME) -config examples/basic-text-overlay.yaml

run-timestamp:
	./$(BUILD_DIR)/$(BINARY_NAME) -config examples/timestamp-overlay.yaml

run-multicast:
	./$(BUILD_DIR)/$(BINARY_NAME) -config examples/multicast-output.yaml

# Development targets
dev: deps build run

# Check GStreamer installation
check-gstreamer:
	@echo "Checking GStreamer installation..."
	@gst-inspect-1.0 --version || (echo "GStreamer not found. Please install GStreamer." && exit 1)
	@gst-inspect-1.0 hlsdemux > /dev/null || (echo "hlsdemux plugin not found. Please install gst-plugins-good." && exit 1)
	@gst-inspect-1.0 x264enc > /dev/null || (echo "x264enc plugin not found. Please install gst-plugins-ugly." && exit 1)
	@gst-inspect-1.0 udpsink > /dev/null || (echo "udpsink plugin not found. Please install gst-plugins-good." && exit 1)
	@echo "GStreamer installation looks good!"

# Test UDP output with VLC
test-udp:
	@echo "Testing UDP output with VLC..."
	@echo "Make sure the application is running, then execute:"
	@echo "vlc udp://@127.0.0.1:5000"

# Docker targets (if needed)
docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm -p 5000:5000/udp $(BINARY_NAME)

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Download dependencies and build"
	@echo "  build            - Build the application"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run tests"
	@echo "  deps             - Download Go dependencies"
	@echo "  run              - Build and run with default config"
	@echo "  config           - Create example configuration file"
	@echo "  dev              - Development build and run"
	@echo "  check-gstreamer  - Check GStreamer installation"
	@echo "  install-deps-*   - Install system dependencies"
	@echo "  run-*            - Run with specific example configs"
	@echo "  test-udp         - Show command to test UDP output"
	@echo "  help             - Show this help message"
