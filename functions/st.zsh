#!/usr/bin/env bash

source "$(dirname "$0")/../src/utils.sh"
script_name='st'

printHelp() {
  usage "$script_name" "[filter] [path1 path2...]" "[--stop]"
  descriptionHeader
  description "Run PHPUnit tests with Sail, with an optional filter phrase, eg. 'testSomething'"
  description "Also accepts any standard PHPUnit options in addition to the filter"
  description "Equivalent to:"
  description "${CYAN} sail test --filter [options]"
  echo

  argumentHeader
  summaryLine "filter" "Optionally include a filter phrase to run specific tests."
  summaryLine "path1 path2..." "Optionally include a list of testCasePaths to restrict the tests to"

  optionHeader
  summaryLine "stop" "Stop on the first failure or error"
}
# printSummary - called when --summary is passed. Should output a one-liner summary of the script's purpose, usually with usageRow
printSummary() {
  # get the description from getDescription
  desc=$(printDescription)
  usageRow "$script_name" "$desc" "[filter] [path1 path2...]" "[--stop]"
}

# getDescription - called when --description is passed. Should output a single-line description of the script's purpose
printDescription(){
  echo "Run PHPUnit tests with Sail, with an optional filter phrase, eg. 'testSomething'"
}

alisRun_st(){
    echo 'running st'
    return 1
  checkArgs "$@" || return 1


  while getopts ":v:-:" opt; do
      case $opt in
          -)
              case ${OPTARG} in
                  stop)
                      stopFlags=true
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

  shift $((OPTIND -1))

  ### Command logic ###
  testCasePaths=""

  # If there are arguments:
  if [ "$#" -gt 0 ]; then
      # If the first argument is not a path, its a filter phrase
      if [[ ! "$1" =~ ^.*\/.*$ ]]; then
          filter="--filter=$1"
          # Shift the filter off the arguments
          shift
      else
          filter=""
          testCasePaths="$@"
          # Shift all arguments off the list
          shift $#
      fi
  fi

  # If --stop was provided, tack on --stop-on-error and --stop-on-failure
  if [ "$stopFlags" = true ]; then
      stopFlags="--stop-on-error --stop-on-failure"
  else
      stopFlags=""
  fi

  bin="vendor/bin/sail"
  # Check that sail is installed
  if [ ! -f "$bin" ]; then
      error "Error: Sail not found. Run from the project root. \n"
      return 1
  fi

  # Construct the runCommand with the quoted filter string and additional options
  runCommand="$bin test $filter $testCasePaths $stopFlags"
  # Append any additional options, excluding the first argument
  for arg in "${@:2}"; do
      runCommand="$runCommand $arg"
  done


  # Execute the final runCommand
  runSummary "$bin test" "" "$filter $testCasePaths" "$stopFlags"
  $runCommand
}
