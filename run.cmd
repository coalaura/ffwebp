@echo off

echo Rebuilding...
go build -tags full -o ffwebp.exe .\cmd\ffwebp

.\ffwebp.exe %*