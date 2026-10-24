#!/bin/sh
# Build-time only; never edits an installed worker or downloads source.
set -eu
source_dir=${1:?usage: apply-patches.sh PINNED_SOURCE_DIRECTORY}
patch_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
expected=b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9
actual=$(git -C "$source_dir" rev-parse HEAD)
if [ "$actual" != "$expected" ]; then
    echo "OffGrid native-sequential patch requires $expected, found $actual" >&2
    exit 1
fi
git -C "$source_dir" diff --exit-code HEAD -- common/chat-auto-parser-generator.cpp common/build-info.cpp.in tests/test-chat-auto-parser.cpp
test ! -e "$source_dir/tests/offgrid-native-sequential.h"
git -C "$source_dir" apply --check "$patch_dir/native-sequential.patch"
git -C "$source_dir" apply "$patch_dir/native-sequential.patch"
cp "$patch_dir/offgrid-native-sequential.h" "$source_dir/tests/offgrid-native-sequential.h"
echo "Applied OffGrid native sequencing revision offgrid-seq2 to $expected"
