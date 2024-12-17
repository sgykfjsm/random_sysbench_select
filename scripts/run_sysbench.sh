#!/usr/bin/env bash

set -eu -o pipefail

table_num=10
table_size=1000

db_num="1"
db_driver="mysql"
mysql_host="127.0.0.1"
mysql_port=4000
mysql_user="root"
mysql_password=""

threads=64
report_interval=10  # seconds

prefix="sbtest"

sysbench_opt="--mysql-host=${mysql_host}"
sysbench_opt="${sysbench_opt} --mysql-port=${mysql_port}"
sysbench_opt="${sysbench_opt} --mysql-user=${mysql_user}"
sysbench_opt="${sysbench_opt} --mysql-password=${mysql_password}"
sysbench_opt="${sysbench_opt} --threads=${threads}"
sysbench_opt="${sysbench_opt} --report-interval=${report_interval}"
sysbench_opt="${sysbench_opt} --db-driver=${db_driver}"

export MYSQL_PWD="${mysql_password}"
mysql_cmd="mysql -sss --host ${mysql_host} --port ${mysql_port} --user ${mysql_user}"

# Help command
if [ "$#" -ge 1 ] && [ "${1}" = "help" ]; then
    echo "Usage: $0 [command]"
    echo "Commands:"
    echo "  help        Show this help message"
    echo "  prepare     Prepare the databases and tables"
    exit 0
fi

cmd="prepare"
workload="oltp_point_select"
if [ "$#" -ge 1 ] && [ "${1}" = "run" ]; then
    workload="oltp_read_write"
fi

echo "$(date "+%FT%T") start CREATE DATABASE"
for i in $(seq 1 ${db_num})
do
    db_name="${prefix}$(printf "%03d" "${i}")"
    q="DROP DATABASE IF EXISTS ${db_name}; CREATE DATABASE ${db_name};"
    echo "$(date "+%FT%T") ${q}"
    ${mysql_cmd} -e "${q}" &
done
wait
echo "$(date "+%FT%T") end CREATE DATABASE"

echo "$(date "+%FT%T") start sysbench ${cmd}"
for i in $(seq 1 ${db_num})
do
    db_name="${prefix}$(printf "%03d" "${i}")"
    sysbench_opt="${sysbench_opt} --mysql-db=${db_name}"

    echo "$(date "+%FT%T") sysbench ${db_name}"
    # shellcheck disable=SC2086
    sysbench ${sysbench_opt} "${workload}" --tables="${table_num}" --table-size="${table_size}" "${cmd}"
done
echo "$(date "+%FT%T") end sysbench ${cmd}"


# EOF
