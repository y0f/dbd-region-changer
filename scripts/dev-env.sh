#!/usr/bin/env bash
# Source this before any build/test that imports Fyne (CGO + OpenGL).
# Pure-Go logic packages do NOT need this (use scripts/test.sh).
export PATH="/c/msys64/mingw64/bin:$PATH"
export CGO_ENABLED=1
export CC=gcc
