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
        new|switch)
            local output
            output=$(wt-bin "$@" --print-path 2>&1)
            local exit_code=$?
            if [ $exit_code -eq 0 ] && [ -d "$output" ]; then
                cd "$output"
            else
                echo "$output" >&2
                return $exit_code
            fi
            ;;
        *)
            wt-bin "$@"
            ;;
    esac
}
`
