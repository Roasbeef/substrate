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

# Check for grpc-gateway tools (optional for now).
GRPC_GATEWAY_AVAILABLE=false
if command -v protoc-gen-grpc-gateway &> /dev/null; then
    GRPC_GATEWAY_AVAILABLE=true
fi

echo "Generating Go code from mail.proto..."

# Generate Go code for protobuf messages and gRPC.
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    mail.proto

echo "Generated protobuf and gRPC stubs."

# Generate grpc-gateway REST proxy if tools are available.
if [ "$GRPC_GATEWAY_AVAILABLE" = true ]; then
    echo "Generating grpc-gateway REST proxy..."

    protoc \
        --grpc-gateway_out=. \
        --grpc-gateway_opt=logtostderr=true \
        --grpc-gateway_opt=paths=source_relative \
        --grpc-gateway_opt=grpc_api_configuration=mail.yaml \
        mail.proto

    echo "Generated grpc-gateway proxy."

    # Generate OpenAPI spec if openapiv2 plugin is available.
    if command -v protoc-gen-openapiv2 &> /dev/null; then
        echo "Generating OpenAPI spec..."

        protoc \
            --openapiv2_out=. \
            --openapiv2_opt=logtostderr=true \
            --openapiv2_opt=grpc_api_configuration=mail.yaml \
            --openapiv2_opt=json_names_for_fields=false \
            mail.proto

        echo "Generated OpenAPI spec."
    else
        echo "Skipping OpenAPI generation (protoc-gen-openapiv2 not found)"
        echo "Install with: go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest"
    fi
else
    echo "Skipping grpc-gateway generation (protoc-gen-grpc-gateway not found)"
    echo "Install with: go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest"
fi

echo ""
echo "Done. Generated files:"
ls -la *.pb.go *.pb.gw.go *.swagger.json 2>/dev/null || echo "  (check for errors)"
