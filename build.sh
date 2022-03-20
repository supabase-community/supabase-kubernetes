#!/bin/bash

helm package ./charts/supabase -d build/
helm repo index ./
# sed 's+build+head+g' ./index.yaml > ./index.yaml

# Crossplatform sed workaround from: https://unix.stackexchange.com/questions/92895/how-can-i-achieve-portability-with-sed-i-in-place-editing
case $(sed --help 2>&1) in
  *GNU*) set sed -i;;
  *) set sed -i '';;
esac

"$@" -e 's+build+https://supabase-community.github.io/supabase-kubernetes/build+g' ./index.yaml
