# Agents

## Bash scripting

When writing bash scripts, ensure they are compatible with the version of bash running on the host. This host uses `bash 3.2.57` (the system bash shipped with macOS), so avoid bash 4+ features such as associative arrays (`declare -A`), `mapfile`/`readarray`, `${var^^}`/`${var,,}` case conversion, and `&>>` redirection.
