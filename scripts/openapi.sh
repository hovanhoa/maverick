#! /bin/bash

for config in $(find . -name "oapi-config*.yaml"); do
  pushd $(dirname $config)
    go run \
    github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
    --config=$(basename $config) \
    $(grep -oE 'spec:.*' $(basename $config) | cut -d ':' -f 2) || exit $?
  popd
done
