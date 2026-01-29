#!/bin/bash
# gen_protos.sh generates Go code from protobuf definitions.
# Based on lnd's lnrpc/gen_protos.sh pattern.

set -e

# Directory containing this script.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check for required tools.
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc not found. Install with: brew install protobuf"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null; then
    echo "Error: protoc-gen-go not found. Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Error: protoc-gen-go-grpc not found. Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

echo "Generating Go code from mail.proto..."

# Generate Go code.
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    mail.proto

echo "Done. Generated files:"
ls -la *.pb.go 2>/dev/null || echo "  (none found - check for errors)"
