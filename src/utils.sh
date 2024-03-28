
# Define color codes
SUCCESS='\033[0;32m'
ERROR='\033[0;31m'
WARNING='\033[0;33m'
INFO='\033[0;34m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color


# Function to print success message
success() {
    echo -e "${SUCCESS}$1${NC}"
}

# Function to print error message
error() {
    echo -e "${ERROR}$1${NC} "
}

info() {
    echo -e "${INFO}$1${NC}"
}

warn() {
     echo -e "${WARNING}$1${NC}"
}

# output a checkmark and indent the message
checkoff() {
    echo -e "    ${SUCCESS}✓${NC} $1"
}

ask(){
  # A warn message followed by a info y/n prompt default y, return true if y or yes
  echo -e "${WARNING}$1${INFO} [y/n] ${NC}"
  read -r confirm

  if [ "$confirm" == "y" ] || [ "$confirm" == "yes" ]; then
    return 1
  fi
  return 0
}

print_header() {
    echo -e "${BLUE}"
    # Ouput contents of "logo" file in the src directory
    cat "$(dirname "$0")/src/logo"
    echo -e "${NC}"

    info "\n\n                               Alias Library In Shell\n"
}
