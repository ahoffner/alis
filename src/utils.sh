# Determine if stdout is a terminal...
if [ -t 1 ]; then
    # Determine if colors are supported...
    if command -v tput >/dev/null 2>&1; then
        ncolors=$(tput colors)

        # Define color codes
        if [ -n "$ncolors" ] && [ "$ncolors" -ge 8 ]; then
            BOLD=$(tput bold)
            RED=$(tput setaf 1)
            GREEN=$(tput setaf 2)
            YELLOW=$(tput setaf 3)
            BLUE=$(tput setaf 4)
            MAGENTA=$(tput setaf 5)
            CYAN=$(tput setaf 6)
            NC=$(tput sgr0)
        fi
    else
        # Fallback: Define default color codes
        BOLD="\033[1m"
        RED="\033[31m"
        GREEN="\033[32m"
        YELLOW="\033[33m"
        BLUE="\033[34m"
        MAGENTA="\033[35m"
        CYAN="\033[36m"
        NC="\033[0m" # No color
    fi
fi



# Loading spinner function with a custom message and inline command execution
runAndWait() {
 local message=$1
  shift
  local command="$@"
  local delay=0.05
  local spinner=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')

  # Run the command in the background, redirecting output to a temp file for error capture
  local temp_output=$(mktemp)
  ( eval "$command" >"$temp_output" 2>&1 ) &

  # Get the PID of the background command
  local pid=$!

  # Show spinner while the command is running
  while kill -0 "$pid" 2>/dev/null; do
    for symbol in "${spinner[@]}"; do
      echo -ne "\r    ${YELLOW}$symbol${NC} $message"
      sleep $delay
    done
  done

  # Check the exit status of the command
  wait $pid
  local exit_status=$?

  # Clear the spinner line
  echo -ne "\r"

  if [[ $exit_status -eq 0 ]]; then
    # Print a success message if command succeeded
    checkoff "$message completed"
  else
    # Print the error output if command failed
    echo -e "    ${RED}✗${NC} Error occurred: "
    cat "$temp_output"
  fi

  # Remove the temporary output file
  rm -f "$temp_output"
}

# Start loading spinner function
startLoading() {
  local message=$1
  local delay=0.1
  local spinner=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')

  # Start a background process to show the spinner
  (
    while true; do
      for symbol in "${spinner[@]}"; do
        echo -ne "\r    ${YELLOW}$symbol${NC} $message"
        sleep $delay
      done
    done
  ) &
  SPINNER_PID=$!  # Store the PID of the spinner process
}

# Stop loading spinner function
stopLoading() {
  # Kill the spinner process
  kill "$SPINNER_PID" 2>/dev/null
  wait "$SPINNER_PID" 2>/dev/null

  # Clear the spinner line
  echo -ne "\r\033[K"
}


# Function to print success message
success() {
  echo -e "${GREEN}$1${NC}"
}

# Function to print error message
error() {
  echo -e "${RED}$1${NC} "
}

info() {
  echo -e "${BLUE}$1${NC}"
}

warn() {
  echo -e "${YELLOW}$1${NC}"
}

# output a checkmark and indent the message
checkoff() {
  echo -e "    ${GREEN}✓${NC} $1"
}

ask() {
  # A warn message followed by a info y/n prompt default y, return true if y or yes
  echo -e "${YELLOW}$1${BLUE} [y/n] ${NC}"
  read -r confirm

  if [ "$confirm" == "y" ] || [ "$confirm" == "yes" ]; then
    return 1
  fi
  return 0
}

# Takes the command string as arg 1, arguments in arg2 and options in arg3 (if any)
usage() {
  command=$1
  args=$2
  # If no third argument, set options to empty string
  if [ -z "$3" ]; then
    options=""
  else
    options=$3
  fi

  echo -e "${YELLOW}Usage:${NC}"
  echo "  ${GREEN}$command ${BLUE}$args ${MAGENTA}$options${NC}"
  echo
}

header() {
  echo -e "${YELLOW}$1${NC}"
}

sectionHeader() {
  # Shows a header with a block of ### above and below
  # Ex ############################
  #    ###        Header        ###
  #    ############################
  # Get the length of the string plus 8 for the padding
  strLength=$((${#1} + 6))
  minStrLength=100

  if [ $strLength -lt $minStrLength ]; then
    strLength=$minStrLength
  fi

  echo -e "${BLUE}"
  printf "%0.s#" $(seq 1 $strLength)
  echo
  # Center the text in the block
  # discount the length of the leading ### and trailing ###
  lineLength=$(($strLength - 6))
  printf "###%*s%*s###\n" $((($lineLength + ${#1}) / 2)) "$1" $((($lineLength - ${#1}) / 2)) ""
  printf "%0.s#" $(seq 1 $strLength)
  echo "${NC}"
}

description() {
  echo "    ${BLUE}$1"
}

descriptionHeader() {
  echo -e "${YELLOW}Description:${NC}"
}

argumentHeader() {
  echo -e "${YELLOW}Arguments:${NC}"
}
optionHeader() {
  echo -e "${YELLOW}Options:${NC}"
}

summaryLine() {
  # Takes two args, the option and a description.
  option="${GREEN}$1"
  description="${NC}$2${NC}"

  # Outputs like usageRow in columns
  printf "%-20s %s\n" "    ${option}" "${description}"

}

aliasUsageRow() {
  # Arg 1 is the alias name, arg 2 is the aliased command and arg 3 is the description
  aliasName="${GREEN}$1"
  command="${BLUE}$2"
  description="${NC}$3${NC}"
  # Print the alias and description in the same line
  displayRow "${aliasName}" "$description"
  # then print the aliased command in the next line with the little down arrow right symbol
  displayRow "" "→ ${command}" 1 4
}

usageRow() {
  # take 2-4 args, the command, description, arguments, and options - the later are optional
  command="${GREEN}$1"
  description="${NC}$2${NC}"

  if [ -z "$3" ]; then
    arguments=""
  else
    arguments=" $3"
  fi
  if [ -z "$4" ]; then
    options=""
  else
    options=" $4"
  fi

  displayRow "${command}${BLUE}${arguments}${MAGENTA}${options}" "$description"
}

displayRow() {
  # Takes two args, the first column and the second column of the row
  firstColumn=$1
  secondColumn=$2

  # If a 3rd arg is provided, it a custom columnWidth, use default
  if [ -z "$3" ]; then
    columnWidth=70
  else
    columnWidth=$3
  fi

  # 4th if supplied is the left padding
  if [ -z "$4" ]; then
    leftPadding=4
  else
    leftPadding=$4
  fi

  # Calculate length of the first column without considering color codes
  length_without_color=$(echo -ne "${firstColumn}" | sed 's/\x1b\[[0-9;]*m//g' | wc -c)

  printf "%-${leftPadding}s %-${columnWidth}s %-${columnWidth}s %b\n" " " "${firstColumn}" "${secondColumn}"

}

runSummary() {
  echo -e "  ${YELLOW}Running:${CYAN}"
  usageRow "$1" "$2" "$3" "$4"
  echo
}

printHeader() {

  echo -e "${BLUE}"
  # Ouput contents of "logo" file in the src directory
  cat "$(dirname "$0")/src/logo"
  echo -e "${NC}"

  info "\n\n                               Alias Library In Shell\n"
}

# Function to check if --help option is provided
# Requires the function define a printHelp function and printSummary function
checkHelp() {
  for arg in "$@"; do
    if [ "$arg" = "--help" ]; then
      return 0
    fi
  done
  return 1
}

checkSummary() {
  for arg in "$@"; do
    if [ "$arg" = "--summary" ]; then
      return 0
    fi
  done
  return 1
}

checkDescription() {
  for arg in "$@"; do
    if [ "$arg" = "--description" ]; then
      return 0
    fi
  done
  return 1
}

checkArgs() {
  if checkHelp "$@"; then
    printHelp
    exit 0
  elif checkSummary "$@"; then
    printSummary
    exit 0
  elif checkDescription "$@"; then
    printDescription
    exit 0
  fi
}
