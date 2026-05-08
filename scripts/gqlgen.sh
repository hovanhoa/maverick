#! /bin/bash
# This script runs all of the graphql generation across all the packages
# in the codebase
set -x

for dir in $(find . -name "gqlgen.yml" -exec dirname {} \;); do
  pushd $dir
    go run github.com/99designs/gqlgen generate || exit $?
  popd
done

for dir in $(find . -name "genqlient.yaml" -exec dirname {} \;); do
  pushd $dir
    go run github.com/Khan/genqlient || exit $?
  popd
done
