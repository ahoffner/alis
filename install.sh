#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Download and extract the tarball
curl -sL https://github.com/ahoffner/alis/archive/refs/tags/1.0.1.tar.gz | tar xz

# Navigate to extracted folder (replace 1.0.0 with your version dynamically if needed)
cd alis-1.0.1

# Run the install command
./alis install

# Clean up the extracted folder
cd ..
rm -rf alis-1.0.1