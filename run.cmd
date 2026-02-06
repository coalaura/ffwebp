@echo off

set CC=zig cc -target x86_64-windows-gnu
set CXX=zig c++ -target x86_64-windows-gnu

if exist ffwebp.exe (
	del ffwebp.exe
)

echo Rebuilding...
go build -tags full -o ffwebp.exe .\cmd\ffwebp

.\ffwebp.exe %*
