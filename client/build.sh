#!/bin/bash


CL="clean"
if [[ "$1" -eq "$CL" ]]; then
  rm -f *_client_terminalchat
  rm -f win_client_terminalchat*
  echo "clean"
  exit 0
fi

# 编译 mac平台
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o macamd_client_terminalchat

# 编译 mac平台 arm架构
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o macarm_client_terminalchat

# 编译 windows平台
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o win_client_terminalchat.exe

# 编译 linux 平台
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o linux_client_terminalchat