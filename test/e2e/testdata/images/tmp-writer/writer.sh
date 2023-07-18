#!/usr/bin/env sh

touch "/tmp/foo.txt" || exit 1

yq '(.desired.resources[] | .resource.metadata.annotations) |= {"tmp-write-access": "true"} + .'
