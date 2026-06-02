#!/bin/bash

set -e

SWAGGER_DIR="gen/openapiv2/cegw/v1"
OUTPUT_FILE="docs/openapi.json"

echo "Merging OpenAPI specifications..."

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed."
    echo "Install with: brew install jq"
    exit 1
fi

# Create docs directory if not exists
mkdir -p docs

# Start with base structure
cat > ${OUTPUT_FILE} << 'EOF'
{
  "swagger": "2.0",
  "info": {
    "title": "CEGW API",
    "version": "1.0.0",
    "description": "Crypto Exchange Gateway - Unified API for cryptocurrency exchange integration"
  },
  "tags": [
    {"name": "Market Data", "description": "Market data operations"},
    {"name": "Trading", "description": "Trading operations"},
    {"name": "Monitoring", "description": "Monitoring and alerts"}
  ],
  "consumes": ["application/json"],
  "produces": ["application/json"],
  "paths": {},
  "definitions": {}
}
EOF

# Merge all swagger files
TEMP_FILE=$(mktemp)

for file in ${SWAGGER_DIR}/*.swagger.json; do
    if [ -f "$file" ]; then
        echo "Merging $(basename $file)..."
        
        # Merge paths
        jq -s '.[0].paths as $base | .[1].paths as $new | .[0] | .paths = ($base + $new)' \
            ${OUTPUT_FILE} "$file" > ${TEMP_FILE}
        mv ${TEMP_FILE} ${OUTPUT_FILE}
        
        # Merge definitions
        jq -s '.[0].definitions as $base | .[1].definitions as $new | .[0] | .definitions = ($base + $new)' \
            ${OUTPUT_FILE} "$file" > ${TEMP_FILE}
        mv ${TEMP_FILE} ${OUTPUT_FILE}
    fi
done

# Pretty print
jq '.' ${OUTPUT_FILE} > ${TEMP_FILE}
mv ${TEMP_FILE} ${OUTPUT_FILE}

echo "OpenAPI specification merged successfully: ${OUTPUT_FILE}"
