#!/bin/bash

source "$(dirname "$0")/../src/utils.sh"
script_name='gsbr'

#### Required Functions ####
# printHelp - called when --help is passed
printHelp() {
  usage "$script_name" "<target branch>" ""
  descriptionHeader
  description "Shorthand for 'git sync branch resolution'"
  description "With git, sync a conflict resolution branch (from gcbr) with both the source and target branches."
  description "The checked out branch will issue a pull."
  description "Equivalent to:"
  description "    ${CYAN}git checkout -b <target-branch> && git pull origin <target-branch> ${BLUE}"
  description "...but with some error handling & recovery."
}
# printSummary - called when --summary is passed. Should output a one-liner summary of the script's purpose, usually with usageRow
printSummary() {
  # get the description from getDescription
  desc=$(printDescription)
  usageRow "$script_name" "$desc" "<target branch>"
}
# getDescription - called when --description is passed. Should output a single-line description of the script's purpose
printDescription(){
  echo "With git, sync a conflict resolution branch (from gcbr) with both the source and target branches."
}
alisRun_gsbr() {
  # Output what we're doing
  runSummary "$script_name" "Syncing a conflict resolution branch (from gcbr) with both the source and target branches."
  
  # Check we're in a git repo
  if ! git rev-parse --is-inside-work-tree &> /dev/null; then
      error "Error: Not in a git repository\n"
      return 1
  fi
  
  checkArgs "$@" || return 1
  
  
  ### Input Processing ###
  # Must have 1 argument
  if [ "$#" -ne 1 ]; then
    # Ask the user to provide a target branch, via input:
    echo "Enter the target branch (we'll combine it and this branch into a new branch): "
    read target_branch
  else
    target_branch="$1"
  fi
  
  if [ -z "$target_branch" ]; then
    error "Error: No target branch provided\n"
    return 1
  fi
  
  
  ### Command logic ###
  current_branch=$(git rev-parse --abbrev-ref HEAD)
  new_branch="${current_branch}-${target_branch}"
  
  # Ensure theres isn't any uncommitted changes
  if ! git diff-index --quiet HEAD --; then
      error "Error: Uncommitted changes in the working directory. Commit or stash these and retry.\n"
      return 1
  fi
  
  # git fetch to grab remotes
  git fetch || return 1
  
  # Ensure the target branch exists
  if ! git show-ref --quiet "refs/remotes/origin/$target_branch"; then
      error "Error: Target branch '$target_branch' does not exist\n"
      return 1
  fi
  
  # Check if the new branch already exists
  wasNew=0
  if git show-ref --quiet "refs/heads/$new_branch"; then
    info "Branch '$new_branch' already exists. Checking it out & updating it..."
    git checkout "$new_branch" || return 1
    git pull origin "$new_branch" || return 1
  else
    info "Creating branch '$new_branch'"
    wasNew=1
      git checkout -b "$new_branch" || return 1
  fi
  echo
  
  info "Pulling changes from '$target_branch' into '$new_branch'..."
  # Always pull changes from the specified target branch
  git pull origin "$target_branch"
  echo
  
  success "Branch '$new_branch' created and pulled in '$target_branch'"
  
  # Check if there are conflicts (there probably should be or this wasnt required), if so print a message
  if git diff --name-only --diff-filter=U | grep -q .; then
      info "There are outstanding conflicts in the branch (likely as expected)."
      info "Resolve them manually, commit, and open a PR from ${new_branch} to ${target_branch} to cleanly merge upstream with conflicts resolved."
  else
    warn "No conflicts found - this action not have been necessary."
  
    # ask if they want to undo this by deleting the branch and checking out the old one
    if [ $wasNew -eq 1 ]; then
      ask "Would you like to undo this action? (Delete the new branch and checkout the original branch)"
      if [ $? -eq 1 ]; then
        git checkout "$current_branch"
        git branch -D "$new_branch"
        success "Branch '$new_branch' deleted. Checked out previous branch '$current_branch'"
      else
        info "Leaving branch '$new_branch' as is."
      fi
    fi
  fi
}
