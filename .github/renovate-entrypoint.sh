#!/bin/sh

curl -fsSLo /usr/local/bin/earthly https://github.com/earthly/earthly/releases/latest/download/earthly-linux-amd64
chmod +x /usr/local/bin/earthly
/usr/local/bin/earthly bootstrap

renovate
