#!/usr/bin/env bash

#set -x

make clean
make
cd bin
7z a goodlink-linux-amd64.7z goodlink-linux-amd64
7z a goodlink-linux-arm64.7z goodlink-linux-arm64
7z a goodlink-windows-amd64.7z goodlink-windows-amd64.exe
