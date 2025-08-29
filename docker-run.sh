#!/bin/bash

# MariaDB Extractor Docker Runner
# This script provides an easy way to run the MariaDB extractor using Docker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to show usage
show_usage() {
    echo "MariaDB Extractor Docker Runner"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  extract    Extract database metadata"
    echo "  ddl        Extract DDL statements"
    echo "  dump       Create database dump"
    echo "  help       Show help for a command"
    echo "  build      Build the Docker image"
    echo "  shell      Start interactive shell in container"
    echo ""
    echo "Examples:"
    echo "  $0 extract --all-databases"
    echo "  $0 ddl --databases mydb"
    echo "  $0 dump --all-databases --compress"
    echo "  $0 build"
    echo "  $0 shell"
    echo ""
    echo "Environment variables:"
    echo "  MARIADB_HOST, MARIADB_PORT, MARIADB_USER, MARIADB_PASSWORD"
    echo ""
    echo "You can set these in a .env file or export them in your shell."
}

# Function to check if Docker is installed
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        print_info "Please install Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi
}

# Function to build the Docker image
build_image() {
    print_info "Building MariaDB Extractor Docker image..."
    docker build -t mariadb-extractor .
    print_success "Docker image built successfully!"
}

# Function to run the tool
run_tool() {
    local cmd="$*"

    # Check if image exists, build if not
    if ! docker image inspect mariadb-extractor &> /dev/null; then
        print_warning "Docker image not found. Building..."
        build_image
    fi

    print_info "Running: mariadb-extractor $cmd"

    # Run the container with environment variables and volume mount
    docker run --rm -it \
        --env-file .env 2>/dev/null || true \
        -e MARIADB_HOST \
        -e MARIADB_PORT \
        -e MARIADB_USER \
        -e MARIADB_PASSWORD \
        -e MARIADB_OUTPUT_PREFIX \
        -v "$(pwd)":/app/output \
        mariadb-extractor \
        $cmd
}

# Function to start interactive shell
start_shell() {
    print_info "Starting interactive shell in container..."

    docker run --rm -it \
        --env-file .env 2>/dev/null || true \
        -e MARIADB_HOST \
        -e MARIADB_PORT \
        -e MARIADB_USER \
        -e MARIADB_PASSWORD \
        -e MARIADB_OUTPUT_PREFIX \
        -v "$(pwd)":/app/output \
        mariadb-extractor \
        /bin/sh
}

# Main script logic
main() {
    check_docker

    case "${1:-help}" in
        "build")
            build_image
            ;;
        "shell")
            start_shell
            ;;
        "help"|"-h"|"--help")
            show_usage
            ;;
        "extract"|"ddl"|"dump")
            shift
            run_tool "$1" "$@"
            ;;
        *)
            print_error "Unknown command: $1"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"