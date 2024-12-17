#!/usr/bin/env bash

set -eu -o pipefail

host="127.0.0.1"
port=4000
user="root"
password=""
output_dir="$(pwd)/export"

tiup dumpling -u "${user}" -p "${password}" -P "${port}" -h "${host}" --filetype csv -t 16 -o "${output_dir}" -r 100 -F 256MiB

# EOF
