#!/bin/sh
cd $(dirname $0)
python3 generatelegacy.py > legacy.go
goimports -w legacy.go
