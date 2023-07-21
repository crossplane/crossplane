#!/usr/bin/env sh

touch "/tmp/foo.txt" || exit 1

yq '(.desired.resources[] | .resource.metadata.labels) |= {"tmp-writer.xfn.crossplane.io": "true"} + .'
