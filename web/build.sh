#!/bin/bash

corepack enable
yarn install
yarn build
EMBED_DIR="../cmd/services/api/dist"
mkdir -p "$EMBED_DIR"
rm -rf "$EMBED_DIR"/*
cp -r dist/* "$EMBED_DIR"/
ls "$EMBED_DIR"