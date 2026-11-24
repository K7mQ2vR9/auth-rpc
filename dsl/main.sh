#!/bin/bash

files=$(find ./dsl/tpl -type f -name '*.proto')

rm -f ./dsl/auth_rpc.proto
echo 'syntax = "proto3";' >> dsl/auth_rpc.proto
echo 'package rpc;' >> dsl/auth_rpc.proto
echo 'option go_package ="./authRpc";' >> dsl/auth_rpc.proto

for file in $files; do

  echo "Processing $file"

  sed '/syntax = "proto3";/d;/^import "tpl\//d;/package rpc;/d;/option go_package = ".\/rpc";/d' "$file" >> dsl/auth_rpc.proto

done