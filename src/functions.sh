
# Define the directory where your scripts and aliases are located
SCRIPTS_DIR="$(dirname "$0")/bin"
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

  exit 1
}

# Run a script from bin
run() {
  SCRIPT_NAME="$1"
  shift # Remove the first argument, leaving any additional arguments

  # Check if the script exists
  if [ ! -f "$SCRIPTS_DIR/$SCRIPT_NAME" ]; then
    echo "Error: script '$SCRIPT_NAME' does not exist."
    exit 1
  fi

  # Call the script with --summary to get the summary, save it to a variable
  description=$("$SCRIPTS_DIR/$SCRIPT_NAME" --description)

  # Run the script with any additional arguments
  "$SCRIPTS_DIR/$SCRIPT_NAME" "$@"

exit
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

  # List all aliases in /aliases. This file will have comments to parse. Lines starting with ## indicate a section header, one one # a description proceeding the alias

  # loop through all files in the aliases directory
  for alias in "$(dirname "$0")/aliases"/*; do
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
            # Grab the command after the =, strip the " and ' and the ; if it exists
            command=$(echo $line | cut -d '=' -f 2 | sed 's/["'\'']//g' | sed 's/;//g')
            aliasUsageRow "$aliasName" "$command" "$description"
            description=""
          fi
        fi
      fi
    done <"$alias"
  done

}

install() {
  # Print ASCII art header
  printHeader

  info "Installing..."

  # Get the absolute path to the alis directory
  ALIS_DIR=$(cd "$(dirname "$0")" && pwd)
  TIMESTAMP=$(date +%s)

  # Do the following in full, if it errors out, we'll print a message and exit
  set -e
  # Add aliases from /aliases to shell configuration file
  # Include a header to make it clear where the aliases are coming from, or find that line and replace contents between it and the footer
  if grep -q "$CONFIG_HEADER" "$CONFIG_FILE"; then
    # Delete everything from the header to the footer
    checkoff "Removing existing alis configuration..."
    sed -i '' "/$CONFIG_HEADER/,/$CONFIG_FOOTER/d" "$CONFIG_FILE"
  fi

  # Add the header, aliases, and footer to the shell configuration file
  echo -e "\\n$CONFIG_HEADER" >>"$CONFIG_FILE"
  echo "$CONFIG_SEE" >>"$CONFIG_FILE"
  echo "source $ALIS_DIR/aliases/*" >>"$CONFIG_FILE"
  echo "export PATH=\"\$PATH:$ALIS_DIR/bin\"" >>"$CONFIG_FILE"
  echo "export PATH=\"\$PATH:$ALIS_DIR\"" >>"$CONFIG_FILE"
  echo "$CONFIG_FOOTER" >>"$CONFIG_FILE"

  checkoff "Aliases added to $CONFIG_FILE"

  # If failed, print error message and exit
  if [ $? -eq 0 ]; then
    success "\n${GREEN}Installation successful!${NC} \n"

    warn "\nRun the following to start using alis:"
    echo
    info "     ${CYAN}source $CONFIG_FILE ${NC}"
    echo
    echo "Then, ${CYAN}alis list${NC} to view all possible commands, or  ${CYAN}alis --help${NC} for usage information."
    echo
    echo ""
  else
    error "An error occurred during installation."
  fi
}

update() {
  # Update the library from git
  # Check if there are any changes upstream
  git fetch
  if [ $(git rev-list HEAD...origin/main --count) -gt 0 ]; then
    # If there are changes, prompt the user to update
    ask "There are updates available. Would you like to update alis?"
    if [ $? -eq 1 ]; then
      # If the user confirms, update the library
      git pull
      install
      success "Alis has been updated."
    else
      # If the user declines, exit
      exit 0
    fi
  else
    # If there are no changes, print a message and exit
    success "Alis is up to date."
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
    exit 0
  fi
}