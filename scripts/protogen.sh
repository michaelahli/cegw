#!/bin/bash

set -e

PROTO_DIR="proto"
OUT_DIR="gen"

mkdir -p ${OUT_DIR}

echo "Generating protobuf code..."

protoc -I ${PROTO_DIR} \
  --go_out ${OUT_DIR} --go_opt paths=source_relative \
  --go-grpc_out ${OUT_DIR} --go-grpc_opt paths=source_relative \
  --grpc-gateway_out ${OUT_DIR} --grpc-gateway_opt paths=source_relative \
  --openapiv2_out ${OUT_DIR}/openapiv2 --openapiv2_opt logtostderr=true \
  ${PROTO_DIR}/cegw/v1/*.proto

echo "Protobuf generation complete!"
