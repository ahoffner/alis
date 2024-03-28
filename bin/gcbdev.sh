gcbdev() {
    current_branch=$(git rev-parse --abbrev-ref HEAD)
    new_branch="${current_branch%-*}-development"

    # Check if the new branch already exists
    if git show-ref --quiet "refs/heads/$new_branch"; then
        git checkout "$new_branch" || return 1
    else
        git checkout -b "$new_branch" || return 1
    fi

    # Always pull changes from the development branch
    git pull origin development || return 1
}