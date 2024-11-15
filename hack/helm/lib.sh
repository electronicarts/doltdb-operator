# Library shell functions.

# vtag prepends "v" to the argument and falls back to "latest".
vtag() {
  if [ -z "$1" ] || [ "$1" = latest ]; then
    printf latest
  else
    printf "v%s" "$1"
  fi
}
