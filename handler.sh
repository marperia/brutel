#!/bin/bash
# remove "telnet://" from telnet link
HOST=$(echo "$1" | sed -e 's/telnet:\/\///')

# Run Brutel
/path/to/the/binary/brutel "$HOST"