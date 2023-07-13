#!/usr/bin/env sh

touch "/tmp/foo.txt"
yq '(.desired.resources[] | .resource.metadata.annotations) |= {"lines": "finally!"} + .'
