#!/usr/bin/env bash

set -eu -o pipefail

output_log="/tmp/tidb-lighting.log"
kv_dir="/tmp/tidb-lightning/kv-dir"
rm -rf "${kv_dir}" "${output_log}" "/tmp/tidb_lightning_checkpoint.pb" "/tmp/nohup.out"
mkdir -pv "${kv_dir}"

nohup tiup tidb-lightning -config tidb-lightning.toml > /tmp/nohup.out 2>&1 &
echo tail -F "${output_log}"


# EOF
