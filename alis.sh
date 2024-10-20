#!/bin/bash

# Ensure aliases are expanded
setopt aliases 2>/dev/null || shopt -s expand_aliases

# The installation directory in zsh
ALIS_DIR="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/alis"

# Includes
source "$ALIS_DIR/src/utils.sh"
source "$ALIS_DIR/src/functions.sh"


echo "opts" 
### Option Processing ###
while getopts ":v-:" opt; do
echo $opt
    case $opt in
        -)
            case ${OPTARG} in
                help)
                    printHelp
                    return 0
                    ;;
                *)
                    echo "Invalid option: --${OPTARG}" >&2
                    return 1
                    ;;
            esac
            ;;
        :)
            echo "Option -$OPTARG requires an argument." >&2
            return 1
            ;;
        ?)
            echo "Invalid option: -$OPTARG" >&2
            return 1
            ;;
    esac
done
return 0

### Arg Processing ###
# If no args are supplied, or they called help or --help display the help message
if [[ -z "$1" ]] || [[ "$1" == "help" ]]; then
  printHelp
# If the first argument prefixed with "alisRun" is a function, run it:
elif declare -F "alisRun_$1" > /dev/null; then
    "alisRun_$@"
    return 0
else
  # Otherwise, attempt to run the specified script if its in bin
  if [ -f "$ALIS_DIR/functions/$1" ]; then
    # Run
    run "$1" "${@:2}"

   #  if it exists OK then print success, fail otherwise
    if [ $? -eq 0 ]; then
      echo
      success "ALIS content - script success"
    else
      echo
      error "ALIS is sad - something went wrong"
    fi
  else
    echo 'sourcing'
    # Source all the aliases.sh in the /aliases.sh directory
    source "$ALIS_DIR/aliases/aliases"

    # If the first argument is an alias, it will now be a function in scope of this script
    if [ -n "$(type -t $1)" ]; then
      # If the alias is found, run it with arguments, stripping the first as its redudant now, and dont include if empty
      command=$1
      echo $command "$@"
      shift
      $command "$@"
      return 0
    else
      # If the alias is not found, print an error message
      error "ALIS is sad - script or alias '$1' does not exist."
      return 1
    fi
  fi
fi