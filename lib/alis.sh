# shellcheck shell=bash
# Helpers for alis pack scripts. Source it at the top of a script:
#
#   source "${ALIS_LIB:?run this through alis}"
#
# Keep this bash 3.2-compatible: it runs under macOS's /bin/bash.

# Colors for scripts to use; empty when output is not a terminal or NO_COLOR is set.
# shellcheck disable=SC2034
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  ALIS_BOLD=$'\033[1m' ALIS_DIM=$'\033[2m' ALIS_RED=$'\033[31m' ALIS_GREEN=$'\033[32m'
  ALIS_YELLOW=$'\033[33m' ALIS_BLUE=$'\033[34m' ALIS_CYAN=$'\033[36m' ALIS_RESET=$'\033[0m'
else
  ALIS_BOLD='' ALIS_DIM='' ALIS_RED='' ALIS_GREEN='' ALIS_YELLOW='' ALIS_BLUE='' ALIS_CYAN='' ALIS_RESET=''
fi

info()    { printf '%s\n' "${ALIS_BLUE}$*${ALIS_RESET}"; }
success() { printf '%s\n' "${ALIS_GREEN}✓ $*${ALIS_RESET}"; }
warn()    { printf '%s\n' "${ALIS_YELLOW}! $*${ALIS_RESET}" >&2; }
error()   { printf '%s\n' "${ALIS_RED}✗ $*${ALIS_RESET}" >&2; }
die()     { error "$@"; exit 1; }

# confirm "Question?" — returns 0 only if the user answers y/yes. Defaults to no.
confirm() {
  local reply
  printf '%s [y/N] ' "${ALIS_YELLOW}$*${ALIS_RESET}" >&2
  read -r reply 2>/dev/null </dev/tty || return 1
  case "$reply" in
    [yY] | [yY][eE][sS]) return 0 ;;
    *) return 1 ;;
  esac
}

# prompt "Label" — reads a line from the terminal and prints it.
prompt() {
  local reply
  printf '%s ' "${ALIS_CYAN}$*${ALIS_RESET}" >&2
  read -r reply 2>/dev/null </dev/tty || return 1
  printf '%s' "$reply"
}

# run cmd args... — echoes the command, dimmed, then runs it.
run() {
  printf '%s\n' "${ALIS_DIM}\$ $*${ALIS_RESET}" >&2
  "$@"
}

require_git_repo() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "Not inside a git repository."
}

require_clean_tree() {
  git diff-index --quiet HEAD -- 2>/dev/null || die "You have uncommitted changes. Commit or stash them first."
}

# has_conflicts — true when the index has unmerged paths.
has_conflicts() {
  [ -n "$(git diff --name-only --diff-filter=U)" ]
}
