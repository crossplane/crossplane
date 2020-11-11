#!/bin/sh
#
# A helper script to restart a given process as part of a Live Update.
#
# Further reading:
# https://docs.tilt.dev/live_update_reference.html#restarting-your-process
#
# Usage:
#   Copy start.sh and restart.sh to your container working dir.
#
#   Make your container entrypoint:
#   ./start.sh path-to-binary [args]
#
#   To restart the container:
#   ./restart.sh
#
# Copied from: https://github.com/tilt-dev/rerun-process-wrapper/blob/master/restart.sh

set -u

touch restart.txt
PID="$(cat process.txt)"
if [ $? -ne 0 ]; then
  echo "unable to read process.txt. was your process started with start.sh?"
  exit 1
fi
kill "$PID"