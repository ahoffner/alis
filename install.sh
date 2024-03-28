#!/bin/bash

# Import src/utils.sh
source "$(dirname "$0")/src/utils.sh"

# Print ASCII art header
print_header


# Determine the appropriate shell configuration file
if [ -f "$HOME/.zshrc" ]; then
  CONFIG_FILE="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
  CONFIG_FILE="$HOME/.bashrc"
else
  info "Supported shell configuration file not found."
  exit 1
fi

INSTALL_DIR="$HOME/.alis"
# Allow install dir to be overridden with --install-dir
if [ "$1" == "--install-dir" ]; then
  INSTALL_DIR="$2"
  shift 2
fi

# Confirm installation directory with ask function:
ask "Install alis to $INSTALL_DIR?"
CONTINUE=$?

if [ "$CONTINUE" != 1 ]; then
  error "Installation cancelled."
  exit 1
fi

info "\nInstalling to $INSTALL_DIR..."

# Get the absolute path to the alis directory
ALIS_DIR=$(cd "$(dirname "$0")" && pwd)
TIMESTAMP=$(date +%s)

# Create .alis directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir "$INSTALL_DIR"
fi

# Do the following in full, if it errors out, we'll print a message and exit
set -e

# If the alis directory already exists, move it to .old[timestamp]

if [ -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR.old$TIMESTAMP"
  mv "$INSTALL_DIR" "$INSTALL_DIR.old$TIMESTAMP"
  checkoff "Backed up existing alis directory to $INSTALL_DIR.old$TIMESTAMP"
fi

# Copy alis script and /bin contents to .alis directory
cp "$ALIS_DIR/alis" "$INSTALL_DIR/"
mkdir -p "$INSTALL_DIR/bin"
cp -r "$ALIS_DIR/bin"/* "$INSTALL_DIR/bin/"
mkdir -p "$INSTALL_DIR/aliases"
cp -r "$ALIS_DIR/aliases" "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/src"
cp -r "$ALIS_DIR/src"/* "$INSTALL_DIR/src/"

checkoff "Installed alis files to $INSTALL_DIR"

# Add aliases from /aliases to shell configuration file
# Include a header to make it clear where the aliases are coming from, or find that line and replace contents between it and the footer
CONFIG_HEADER="### MANAGED BY ALIS DO NOT EDIT ####"
CONFIG_SEE="### See https://github.com/ahoffner/alis for more information ####"
CONFIG_FOOTER="### END ALIS MANAGED ALIASES ####"
# Temporary file
temp_file=$(mktemp)

if grep -q "$CONFIG_HEADER" "$CONFIG_FILE"; then
  # Replace the contents between the header and footer with the new aliases and add the alis command to the path

    awk -v header="$CONFIG_HEADER" -v footer="$CONFIG_FOOTER" -v see="$CONFIG_SEE" -v install_dir="$INSTALL_DIR" '
        BEGIN { found=0 }
        $0 ~ header { found=1; print $0; print see; print "source " install_dir "/aliases/*"; print "export PATH=\"\$PATH:" install_dir "/bin\"" }
        !found { print $0 }
        $0 ~ footer { found=0 }
    ' "$CONFIG_FILE" > "$temp_file"
else
  # Add the header, aliases, and footer to the shell configuration file
  echo -e "\\n$CONFIG_HEADER" >> "$CONFIG_FILE"
  echo "$CONFIG_SEE" >> "$CONFIG_FILE"
  echo "source $INSTALL_DIR/aliases/*" >> "$CONFIG_FILE"
  echo "export PATH=\"\$PATH:$INSTALL_DIR/bin\"" >> "$CONFIG_FILE"
  echo "export PATH=\"\$PATH:$INSTALL_DIR/alis\"" >> "$CONFIG_FILE"
  echo "$CONFIG_FOOTER" >> "$CONFIG_FILE"
fi

checkoff "Aliases added to $CONFIG_FILE"


# If failed, print error message and exit
if [ $? -eq 0 ]; then
  success "\nInstallation successful! \n"

  # If there was a backup dir, ask if they want to remove it
  if [ -d "$INSTALL_DIR.old$TIMESTAMP" ]; then
    ask "Remove backup directory $INSTALL_DIR.old$TIMESTAMP?"
    CONTINUE=$?

    if [ "$CONTINUE" == 1 ]; then
      rm -rf "$INSTALL_DIR.old$TIMESTAMP"
      checkoff "Removed backup directory $INSTALL_DIR.old$TIMESTAMP"
    fi
  fi

  info "\nRun 'source $CONFIG_FILE' to start using alis."
  info "Then, run alis --help for usage information."

else
  error "An error occurred during installation."
  # Revert the old directory if there is one
  if [ -d "$INSTALL_DIR.old$TIMESTAMP" ]; then
    mv "$INSTALL_DIR.old$TIMESTAMP" "$INSTALL_DIR"
  fi
fi
