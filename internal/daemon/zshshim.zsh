# Portty zsh shell integration (OSC 7 working-directory reports).
# ZDOTDIR is restored first, so every later startup file (.zprofile, .zshrc,
# .zlogin) and /etc/zshrc's HISTFILE are looked up exactly as without Portty.
if [[ -n "$PORTTY_ZSH_ZDOTDIR" ]]; then
  'builtin' 'export' ZDOTDIR="$PORTTY_ZSH_ZDOTDIR"
else
  'builtin' 'unset' 'ZDOTDIR'
fi
'builtin' 'unset' 'PORTTY_ZSH_ZDOTDIR'
{
  'builtin' 'typeset' _portty_f=${ZDOTDIR-$HOME}"/.zshenv"
  [[ ! -r "$_portty_f" ]] || 'builtin' 'source' '--' "$_portty_f"
} always {
  'builtin' 'unset' '_portty_f'
  if [[ -o interactive ]]; then
    __portty_osc7() {
      'builtin' 'emulate' '-L' 'zsh'
      (( ZSH_SUBSHELL )) && return 0
      'builtin' 'setopt' 'extendedglob'
      'builtin' 'local' 'LC_ALL=C'
      'builtin' 'printf' '\e]7;file://%s\a' "${PWD//(#m)[^A-Za-z0-9\/._~-]/%${(l:2::0:)$(( [##16] #MATCH ))}}" 2>/dev/null >/dev/tty
      return 0
    }
    # Re-asserted at every prompt: a startup file that resets chpwd_functions, or a
    # handoff that lost the daemon's state, heals at the next prompt.
    __portty_precmd() {
      if (( ! ${+chpwd_functions[(r)__portty_osc7]} )); then chpwd_functions+=(__portty_osc7); fi
      __portty_osc7
    }
    if (( ! ${+chpwd_functions[(r)__portty_osc7]} )); then chpwd_functions+=(__portty_osc7); fi
    if (( ! ${+precmd_functions[(r)__portty_precmd]} )); then precmd_functions+=(__portty_precmd); fi
    __portty_osc7
  fi
}
