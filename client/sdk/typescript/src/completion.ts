import type { Command } from "commander";

function subcommandWords(command: Command): string[] {
  return command.commands.map((item) => item.name());
}

export function renderCompletion(shell: string, program: Command): string {
  const topLevel = subcommandWords(program).join(" ");
  const openai = subcommandWords(program.commands.find((item) => item.name() === "openai") ?? program).join(" ");
  const anthropic = subcommandWords(program.commands.find((item) => item.name() === "anthropic") ?? program).join(" ");
  const gemini = subcommandWords(program.commands.find((item) => item.name() === "gemini") ?? program).join(" ");

  switch (shell) {
    case "bash":
      return `# bash completion for onr-sdk-ts
_onr_sdk_ts_completions() {
  local cur prev words cword
  _init_completion || return
  if [[ $cword -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${topLevel}" -- "$cur") )
    return
  fi
  case "\${words[1]}" in
    openai)
      COMPREPLY=( $(compgen -W "${openai}" -- "$cur") )
      ;;
    anthropic)
      COMPREPLY=( $(compgen -W "${anthropic}" -- "$cur") )
      ;;
    gemini)
      COMPREPLY=( $(compgen -W "${gemini}" -- "$cur") )
      ;;
  esac
}
complete -F _onr_sdk_ts_completions onr-sdk-ts
`;
    case "zsh":
      return `#compdef onr-sdk-ts
_onr_sdk_ts() {
  local -a commands
  commands=(${topLevel})
  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi
  case "$words[2]" in
    openai)
      _describe 'openai command' "(${openai})"
      ;;
    anthropic)
      _describe 'anthropic command' "(${anthropic})"
      ;;
    gemini)
      _describe 'gemini command' "(${gemini})"
      ;;
  esac
}
compdef _onr_sdk_ts onr-sdk-ts
`;
    case "fish":
      return `complete -c onr-sdk-ts -f
complete -c onr-sdk-ts -n "__fish_use_subcommand" -a "${topLevel}"
complete -c onr-sdk-ts -n "__fish_seen_subcommand_from openai" -a "${openai}"
complete -c onr-sdk-ts -n "__fish_seen_subcommand_from anthropic" -a "${anthropic}"
complete -c onr-sdk-ts -n "__fish_seen_subcommand_from gemini" -a "${gemini}"
`;
    default:
      throw new Error(`unsupported shell: ${shell}`);
  }
}
