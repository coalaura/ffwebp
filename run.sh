#!/bin/bash

echo "Rebuilding..."
go build -tags full -o ffwebp ./cmd/ffwebp

chmod +x ffwebp

./ffwebp $*
