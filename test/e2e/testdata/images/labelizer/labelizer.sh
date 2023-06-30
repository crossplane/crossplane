#!/usr/bin/env sh

yq '(.desired.resources[] | .resource.metadata.labels) |= {"labelizer.xfn.crossplane.io/processed": "true"} + .'
