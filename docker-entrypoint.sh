#!/bin/sh
set -eu

if [ "$(id -u)" = "0" ]; then
  exec gosu app /app/server "$@"
fi

exec /app/server "$@"
