#!/bin/bash

echo "Rebuilding..."
go build -tags full,play -o ffwebp ./cmd/ffwebp

chmod +x ffwebp

./ffwebp $*
