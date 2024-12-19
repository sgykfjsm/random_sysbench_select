# Random Select for Sysbench

This Go program executes random SELECT queries against [Sysbench](https://github.com/akopytov/sysbench) test database. It simulates database load by generating random query conditions, controlling QPS (Queries Per Second), and reporting detailed performance metrics such as success rates, failure rates, and query latencies.

In most cases, `sysbench run` command meet the requirement. But the sysbench's query pattern is limited and sometime you need more query pattern randomly. This tool is supposed to expected such a case.  particularly useful for stress testing and benchmarking database performance under controlled query loads.

# Features
- Random Query Generations: Fields, tables, and conditions are generated randomly.
- Multiple Databases and Tables: Supports running queries across multiple databases and tables.
- QPS Control: Allows you to throttle the query execution rate.
- Result Reporting: Outputs results in JSON, YAML, or Table format.
- CSV Data Integrations: Random values for query conditions are picked from CSV files.

# Installation
```shell
git clone https://gitlab.com/shigeyuki.fujishima/scripts.git
cd scripts/go/random_select_for_sysbench
go build -o random_select
```

# Prerequisites
- Go 1.20 or later
- MySQL/TiDB database with Sysbench test tables
    - For TiDB user: https://docs.pingcap.com/tidb/stable/benchmark-tidb-using-sysbench#how-to-test-tidb-using-sysbench
- CSV data files matching table naming conventions

## Note
- CSV naming convention: The script expects CSV files to follow the naming convention such as `sbtest001.<table_name>.*.csv`.
- Database tables: This script assumes tables follow the Sysbench schema.
- High load scenarios: Be cautious with QPS settings, as high values may overwhelm your database.

## Data preparation

This program is supposed to execute SELECT queries to Sysbench database. Therefore, you need to prepare databases with Sysbench. If you are not familiar to Sysbench, take a look at the official README at https://github.com/akopytov/sysbench/blob/master/README.md. If you are fine to read the shell script, please check [this script](./scripts/run_sysbench.sh).

The following steps show how to export the table records as CSV with [dumpling](https://docs.pingcap.com/tidb/stable/dumpling-overview).
```shell
$ tiup dumpling -u root -P 4000 -h 127.0.0.1 --filetype CSV -t 8 -o ./csv -r 200000 -F 256MiB
```

This command exports table data from the database into CSV files. The output is split into chunks (maximum size: 256MiB) for efficient handling.

You don't have to use dumpling, but you have to follow [the naming convention adopted by TiDB lightning](https://docs.pingcap.com/tidb/stable/tidb-lightning-data-source). Using dumpling is the easiest way to follow the rule. For further details, read https://docs.pingcap.com/tidb/stable/tidb-lightning-data-source

As a result, your input folder should be like following.
```
csv/
├── sbtest001-schema-create.sql
├── sbtest001.sbtest1-schema.sql
├── sbtest001.sbtest1.0000000010000.csv
├── sbtest001.sbtest10-schema.sql
├── sbtest001.sbtest10.0000000010000.csv
├── sbtest001.sbtest100-schema.sql
├── sbtest001.sbtest100.0000000010000.csv
...
```

To summarize the instructions, you can follow these steps.
1. Install required tools such as TiUP, dumpling, MySQL server etc
2. Launch the database (MySQL/TiDB)
3. Prepare the database to export the data using sysbench.
4. Export the database with CSV format using dumpling or others.
5. Run the script with the csv directory you exported the data.

# Usage

```shell
$ ./random_select -help
Usage of ./random_select:
  -addr string
        address to access the database server (default "127.0.0.1:4000")
  -csv string
        path to CSV directory (default "in")
  -dbnum int
        upper limit number of the database (default 50)
  -debug
        if true, a logging level becomes DEBUG
  -duration int
        time of second to keep running the process (default 10)
  -format string
        format of printing the job result. Support json, yaml and table (default "json")
  -password string
        password to access the database server
  -qps int
        expected QPS to execute the query (default 1000)
  -tablenum int
        upper limit number of the table in a single database (default 1000)
  -user string
        db user name (default "root")
  -worker int
        number of workers to run the job (default 10)
```

## Examples

```shell
$ ./random_select -qps 500 -duration 20 -csv testdata/in -dbnum 1
2024/12/17 14:08:34 INFO loading CSV file list
2024/12/17 14:08:35 INFO saved CSV data into cache count=1000
2024/12/17 14:08:35 INFO prepare the database tasks
2024/12/17 14:08:35 INFO opened the database connection
2024/12/17 14:08:35 INFO prepare job workers
2024/12/17 14:08:35 INFO launching 10 workers
2024/12/17 14:08:35 INFO finished to prepare job workers
2024/12/17 14:08:35 INFO set QPS as a rate limit qps=500
2024/12/17 14:08:35 INFO start the job for 20 seconds
2024/12/17 14:08:55 INFO waiting for all job completion
{
  "started_at": "2024-12-17T14:08:35.364963+09:00",
  "ended_at": "2024-12-17T14:08:55.364777+09:00",
  "total_queries": 10500,
  "total_success": 10500,
  "total_failure": 0,
  "success_rate": 1,
  "error_rate": 0,
  "qps": 524.9972765766278,
  "query_latency_p95": 419,
  "query_latency_p90": 564,
  "query_latency_p80": 795,
  "query_latency_p50": 1113
}
2024/12/17 14:08:55 INFO good bye
```
