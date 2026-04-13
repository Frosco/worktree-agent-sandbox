package shell

// GenerateInit generates shell initialization script for the given shell
func GenerateInit(shell string) string {
	switch shell {
	case "bash", "zsh":
		return bashInit
	default:
		return "# Shell '" + shell + "' not supported. Use bash or zsh.\n"
	}
}

const bashInit = `# wt shell integration
# Add to your ~/.bashrc or ~/.zshrc:
#   eval "$(wt-bin shell-init bash)"

wt() {
    case "$1" in
        switch)
            # Check if this might be interactive (switch with no branch argument)
            # Interactive mode needs direct TTY access, so we can't use command substitution
            if [ "$1" = "switch" ] && [ $# -eq 1 ]; then
                # Interactive mode: run directly, write path to temp file
                local tmpfile
                tmpfile=$(mktemp)
                wt-bin switch --print-path > "$tmpfile"
                local exit_code=$?
                if [ $exit_code -eq 0 ]; then
                    local output
                    output=$(cat "$tmpfile")
                    rm -f "$tmpfile"
                    if [ -d "$output" ]; then
                        cd "$output"
                    fi
                else
                    rm -f "$tmpfile"
                    return $exit_code
                fi
            else
                # Non-interactive: can safely capture output
                local output
                output=$(wt-bin "$@" --print-path 2>&1)
                local exit_code=$?
                if [ $exit_code -eq 0 ] && [ -d "$output" ]; then
                    cd "$output"
                else
                    echo "$output" >&2
                    return $exit_code
                fi
            fi
            ;;
        *)
            wt-bin "$@"
            ;;
    esac
}
`
