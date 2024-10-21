#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# ZDH plugin directory
PLUGIN_DIR="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/alis"

# Download and extract the tarball
curl -sL https://github.com/ahoffner/alis/archive/refs/tags/2.0.0.tar.gz | tar xz

# Navigate to extracted folder
cd alis-2.0.0

# Create the plugin directory if it doesn't exist
mkdir -p "$PLUGIN_DIR"

# Copy the plugin files to the plugin directory
cp -r . "$PLUGIN_DIR"

# Clean up the extracted folder
cd ..
rm -rf alis-2.0.0

echo "alis plugin installed successfully to $PLUGIN_DIR"
echo "Please add 'alis' to the plugins array in your .zshrc file"
