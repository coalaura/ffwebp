@echo off

if not exist bin (
    mkdir bin
)

echo Building...
go build -o bin/ffwebp.exe

echo Done