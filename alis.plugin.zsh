# alis.plugin.zsh
#
## Source all function files
for file in $ZSH_CUSTOM/plugins/alis/functions/*.zsh; do
  source $file
done

# Source all alias files
for file in $ZSH_CUSTOM/plugins/alis/aliases/*.sh; do
  source $file
done



# If set to hide this help text with ALIS_HIDE_HELP, dont show it
if [[ "$ALIS_HIDE_STARTUP" != "true" ]]; then
    ALISGREEN=$(tput setaf 2)
    ALISCYAN=$(tput setaf 6)
    ALISBLUE=$(tput setaf 5)
    ALISNC=$(tput sgr0)
    print "${ALISGREEN} 🚀 ALIS Available ${ALISBLUE}[[ Run ${ALISCYAN}alis list${ALISBLUE} to show all available commands. ]]\n${ALISNC}"
fi
