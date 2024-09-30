#!/usr/bin/env python3
import os
import sys

with open("old-types.txt") as f:
  old_types = [line.rstrip("\n") for line in f]
with open("old-enums.txt") as f:
  old_enums = [line.rstrip("\n") for line in f]

os.chdir("../../proto")

new_protos = {}
for dir in os.listdir("."):
  if not dir.startswith("wa"):
    continue
  for file in os.listdir(dir):
    if file.endswith(".pb.go"):
      with open(f"{dir}/{file}") as f:
        new_protos[dir] = f.read()
      break

match_type_map = {
  "HandshakeServerHello": "HandshakeMessage_ServerHello",
  "HandshakeClientHello": "HandshakeMessage_ClientHello",
  "HandshakeClientFinish": "HandshakeMessage_ClientFinish",
  "InteractiveMessage_Header_JpegThumbnail": "InteractiveMessage_Header_JPEGThumbnail",
}

print("// DO NOT MODIFY: Generated by generatelegacy.sh")
print()
print("package proto")
print()
print("import (")
for proto in new_protos.keys():
  print(f'\t"github.com/snaril/whatsmeow/proto/{proto}"')
print(")")
print()

print("// Deprecated: use new packages directly")
print("type (")
for type in old_types:
  match_type = match_type_map.get(type, type)
  for mod, proto in new_protos.items():
    if f"type {match_type} " in proto:
      print(f"\t{type} = {mod}.{match_type}")
      break
    elif f"type ContextInfo_{match_type} " in proto:
      print(f"\t{type} = {mod}.ContextInfo_{match_type}")
      break
  else:
    print(f"{type} not found")
    sys.exit(1)
print(")")
print()

print("// Deprecated: use new packages directly")
print("const (")
for type in old_enums:
  for mod, proto in new_protos.items():
    if f"\t{type} " in proto:
      print(f"\t{type} = {mod}.{type}")
      break
    elif f"\tContextInfo_{type} " in proto:
      print(f"\t{type} = {mod}.ContextInfo_{type}")
      break
  else:
    print(f"{type} not found")
    sys.exit(1)
print(")")
