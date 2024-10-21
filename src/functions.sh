
# Define the directory where your scripts and aliases.sh are located
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do
  DIR="$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
SRC_DIR="$( cd -P "$( dirname "$SOURCE" )" >/dev/null 2>&1 && pwd )/"

BASE_DIR="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/alis"
SCRIPTS_DIR="$BASE_DIR/functions"
ALIASES_DIR="$BASE_DIR/aliases"


CONFIG_SEE="### See https://github.com/ahoffner/alis for more information ####"
CONFIG_HEADER="### MANAGED BY ALIS DO NOT EDIT ####"
CONFIG_FOOTER="### END ALIS MANAGED ALIASES ####"

# Determine the appropriate shell configuration file
if [ -f "$HOME/.zshrc" ]; then
  CONFIG_FILE="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
  CONFIG_FILE="$HOME/.bashrc"
else
  info "Supported shell configuration file not found, must be either .zshrc or .bashrc"
  exit 1
fi


# Print a help message
printHelp() {

  printHeader

  usage "alis ${BOLD}${NC}COMMAND" "<arguments> [options]"

  info "Or, run any of the following commands/aliased directly:"
  usageRow "${BOLD}${NC}COMMAND" "" "<arguments>" "[options]"
  info "     → Ex. ${BOLD}${NC}st ${CYAN}testUser tests/Unit/Integration${NC}"

  echo
  header "Show All Commands:"
  usageRow "list" "List all available scripts and aliases" "" ""

  echo
  header "Setup:"
  usageRow "install" "Adds all the ${CYAN}alis${NC} scripts to your path so they can be invoked directly" "source"
  usageRow "silenceStartup" "Hides the startup message shown when your terminal boots up"

}

# Run a script from bin
run() {
  SCRIPT_NAME="$1"
  shift # Remove the first argument, leaving any additional arguments

  # Check if the script exists
  if [ ! -f "$SCRIPTS_DIR/$SCRIPT_NAME" ]; then
    echo "Error: script '$SCRIPT_NAME' does not exist."
    return 1

  fi

  # Call the script with --summary to get the summary, save it to a variable
  description=$("$SCRIPTS_DIR/$SCRIPT_NAME" --description)

  # Run the script with any additional arguments
  "$SCRIPTS_DIR/$SCRIPT_NAME" "$@"
  
  # Execute the script with any additional arguments, and run it in the background
  "$SCRIPTS_DIR/$SCRIPT_NAME" "$@" &
}

# List all scripts in the bin directory
list() {
  echo

  header "Scripts:"
  for script in "$SCRIPTS_DIR"/*; do
    # Output the --summary for each script
    "$script" --summary
  done
  # Output that you can call --help on these for more
  echo
  info "    ↑ Run any of the above scripts with --help for more information."

  # List all aliases.sh in /aliases.sh. This file will have comments to parse. Lines starting with ## indicate a section header, one one # a description proceeding the alias

  # loop through all files in the aliases.sh directory
  for alias in "$ALIASES_DIR"/*; do
    # Read the file contents line by line
    while IFS= read -r line; do
      # If the line starts with more than two ##, ignore it
      if [[ $line == "### "* ]]; then
        continue
      fi
      # If the line starts with two ##, grab it as a header
      if [[ $line == "## "* ]]; then
        echo
        headerLine=$(echo $line | cut -c 4-)
      fi
      # If the line starts with a single #, grab it as a description to a variable. the line after will be the command
      if [[ $line == "# "* ]]; then
        description=$(echo $line | cut -c 3-)
      fi
      # If the line is not a comment,
      if [[ $line != "# "* ]]; then
        #  and header is set, output the header
        if [[ ! -z $headerLine ]]; then
          header "$headerLine:"
          headerLine=""
        fi
        # and if description is set, print the description and the alias using usageRow
        if [[ ! -z $description ]]; then
          # If this contains "alias [name]=", grab the name of the line alias [name]= and before the '
          if [[ $line == *"alias "* ]]; then
            aliasName=$(echo $line | cut -d ' ' -f 2 | cut -d '=' -f 1)
            # Grab the command after the "=", strip the quotes, and semicolons
            command=$(echo "$line" | cut -d '=' -f 2 | sed 's/["'\'']//g' | sed 's/;//g')

            aliasUsageRow "$aliasName" "$command" "$description"
            description=""
          fi
        fi
      fi
    done <"$alias"
  done

}

# Uninstall the alis script and remove related files
uninstall() {
  # Print ASCII art header
  printHeader

  info "Uninstalling..."

  # Define the target directories
  TARGET_DIR="$BASE_DIR"
  BIN_DIR="$HOME/.local/functions"

  # Remove the alis directory
  if [ -d "$TARGET_DIR" ]; then
    rm -rf "$TARGET_DIR"
    checkoff "Removed $TARGET_DIR"
  else
    warn "$TARGET_DIR does not exist"
  fi

  # Remove the symbolic links in ~/.local/functions
  if [ -L "$BIN_DIR/alis" ]; then
    rm "$BIN_DIR/alis"
    checkoff "Removed symbolic link $BIN_DIR/alis"
  else
    warn "Symbolic link $BIN_DIR/alis does not exist"
  fi

  success "\n${GREEN}Uninstallation successful!${NC} \n"
}

# Print the current version and the most recent available version
version() {
  # Get the current version from the VERSION file
  if [ -f "$BASE_DIR/VERSION" ]; then
    current_version=$(cat "$BASE_DIR/VERSION")
  else
    current_version="0.0.0"  # Default to "0.0.0" if no VERSION file exists
  fi


  # Start loading spinner while querying the latest version
  startLoading "Checking for the most recent version..."

  # Run the command to get the latest version
  latest_tag=$(curl -s https://api.github.com/repos/ahoffner/alis/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

  # Stop the loading spinner
  stopLoading

  if [ -n "$latest_tag" ]; then
    # Compare the versions to check if the current version is up to date
    if [ "$(printf '%s\n%s' "$current_version" "$latest_tag" | sort -V | tail -n 1)" == "$current_version" ]; then
      checkoff "${GREEN}Alis is up to date (version ${CYAN}$current_version${NC})"
    else
      info "You are currently on version ${RED}$current_version${NC}"
      warn "A new version (${GREEN}$latest_tag${YELLOW}) is available"
      echo
      info "Run ${CYAN}alis update${BLUE} to update to the latest version."
    fi
  else
    echo "Unable to determine the most recent available version."
  fi
}


update() {
  # Get the latest release version from GitHub
  latest_tag=$(curl -s https://api.github.com/repos/ahoffner/alis/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

  # Get the current version (assuming it's stored in a file named VERSION)
  if [ -f "$BASE_DIR/VERSION" ]; then
    current_version=$(cat "$BASE_DIR/VERSION")
  else
    current_version="0.0.0"  # Default to "0.0.0" if no VERSION file exists
  fi

  # Compare the versions using sort -V to ensure correct comparison
  if [ "$(printf '%s\n%s' "$current_version" "$latest_tag" | sort -V | tail -n 1)" != "$current_version" ]; then
    # If current version is older than the latest version, prompt the user to update
    ask "There is a new version ($latest_tag) available. You are currently on version $current_version. Would you like to update?"
    if [ $? -eq 1 ]; then
      # If the user confirms, download and install the new version
      temp_dir=$(mktemp -d)
      curl -sL "https://github.com/ahoffner/alis/archive/refs/tags/$latest_tag.tar.gz" | tar -xz -C "$temp_dir"

      # Change to the extracted directory
      cd "$temp_dir/alis-$latest_tag"

      # Run install function from the new version
      ./alis install

      # Update the VERSION file
      echo "$latest_tag" > "$BASE_DIR/VERSION"

      success "Alis has been updated to version $latest_tag."

      # Clean up temporary directory
      rm -rf "$temp_dir"
    else
      # If the user declines, exit
      return 0
    fi
  else
    # If there are no changes, print a message and exit
    success "Alis is up to date (version $current_version)."
  fi
}

silenceStartup() {
  # Check if ALIS_HIDE_STARTUP is set already in $CONFIG_FILE and if so, print that its already hidden
  if ! grep -q "ALIS_HIDE_STARTUP" "$CONFIG_FILE"; then
    # If not, add it IN BETWEEN the $CONFIG_HEADER and $CONFIG_FOOTER
    SILENCE="export ALIS_HIDE_STARTUP=true"
    awk -v footer="$CONFIG_SEE" -v silence="$SILENCE" '{
        print;
        if ($0 == footer) print silence
    }' $CONFIG_FILE > $CONFIG_FILE.tmp && mv $CONFIG_FILE.tmp $CONFIG_FILE
    checkoff "Help messages hidden"
  else
    error "Already set to hide the startup message, nothing to do 👍"
    return 0
  fi
}
