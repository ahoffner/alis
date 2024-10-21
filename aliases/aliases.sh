## Alis
# Alis is a simple alias manager for bash. It allows you to easily manage your aliases in a single file.
alias alis="source $ZSH_CUSTOM/plugins/alis/alis.sh"
alias reload=s"ource ~/.zshrc"

## Regular Aliases
### Run alis list to view in terminal

# Switch ls to default to also show file sizes and colors
alias ls='ls -GFh'

## Git
# Checkout the last branch you were on
alias gco='git checkout -'
# Checkout a branch and pull it
alias gc='git checkout $1 && git pull'
# Apply only the last stash
alias gsal='git stash apply stash@{0}'

## Docker
# Open a shell in the docker container as the root user
alias dockroot="docker exec -u root -t -i nursegridapi /bin/bash"

## Sail
# Set the sail alias to run the sail script
alias sail='sh $([ -f sail ] && echo sail || echo vendor/bin/sail)'
# Start the docker containers in detached mode
alias sup="sail up -d"
# Shorthand for Laravel Sail
alias sail='[ -f sail ] && sh sail || sh vendor/bin/sail'

## Artisan
# Run Laravel database migrations
alias sam="sail artisan migrate"
# Shortcut to run Laravel Artisan [COMMAND]
alias san='sail artisan'
# Shortcut to run sail composer [COMMAND]
alias sac='sail composer'
# Shortcut to run sail artisan tinker
alias tink='sail artisan tinker'

## Env Switching
# Switch the Laravel environment to the local docker environment
alias swl='sail artisan switch:env docker'
# Switch the Laravel environment to the development environment
alias swd='sail artisan switch:env development'
# Switch the Laravel environment to the stage environment
alias sws='sail artisan switch:env stage'

## CS Fixer
# Run PHP CS Fixer
alias cs="php vendor/bin/php-cs-fixer fix --config=.php-cs-fixer.php  --diff --allow-risky=yes";

## Phan
# Run Phan locally on all files
alias phan="php vendor/bin/phan --progress-bar  -k.phan/config.php -C"

## CS + Phan
# Shortcut to run both CS Fixer and Phan locally
alias prep="cs && phanall"

## PHPUnit
# Shortcut to run PHPUnit locally, outside of docker
alias p='vendor/bin/phpunit'

### Alis Functions ###
# Wrap the functions with alis to handle the argument processing
alias gcbr="alis gcbr"
alias gcb="alis gcb"
alias st="alis st"