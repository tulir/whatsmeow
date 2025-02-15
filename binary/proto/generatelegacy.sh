#!/bin/sh
cd "$(dirname "$0")" || exit
python3 generatelegacy.py >legacy.go
goimports -w legacy.go
