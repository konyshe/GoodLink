#!/usr/bin/env bash

#set -x

make clean
make
cd bin
zip goodlink-linux-amd64-cmd.zip goodlink-linux-amd64-cmd
zip goodlink-linux-arm64-cmd.zip goodlink-linux-arm64-cmd
zip goodlink-darwin-amd64-cmd.zip goodlink-darwin-amd64-cmd
zip goodlink-darwin-arm64-cmd.zip goodlink-darwin-arm64-cmd
zip goodlink-linux-386-cmd.zip goodlink-linux-386-cmd
zip goodlink-linux-arm-cmd.zip goodlink-linux-arm-cmd
zip goodlink-linux-armv7l-cmd.zip goodlink-linux-armv7l-cmd
zip goodlink-windows-amd64-cmd.zip goodlink-windows-amd64-cmd.exe
zip goodlink-windows-arm64-cmd.zip goodlink-windows-arm64-cmd.exe
zip goodlink-windows-amd64-ui.zip goodlink-windows-amd64-ui.exe
