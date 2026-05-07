#!/usr/bin/env bash
# Limpiar variables MSYS2 que pueden interferir y ejecutar el ps1
unset MSYSTEM MINGW_PREFIX MSYSTEM_PREFIX MSYSTEM_CHOST MSYSTEM_CTYPE
unset ORIGINAL_PATH ORIGINAL_TEMP ORIGINAL_TMP
exec pwsh -NoProfile -Command 'ceftop.ps1'
