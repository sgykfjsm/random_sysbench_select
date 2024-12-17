This folder stores some helper scripts.

- [export_by_dumpling.sh](./export_by_dumpling.sh): the script to run dumpling
- [run_sysbench.sh](./run_sysbench.sh): batch script to create database and run `sysbench prepare` command.
- [run_tidb-lightning.sh]: the script to run TiDB lightning.


To run the script, you need to install tools like dumpling and TiDB lightning. To mange these tools, I recommend to install [TiUP](https://docs.pingcap.com/tidb/stable/tiup-overview#install-tiup).
After installing TiUP, you can install dumpling and TiDB lightning via following commnad.
- dumping: `tiup install dumpling`
- TiDB lightning: `tiup install tidb-lightning`

You can use [testdata/in.tar.gz](../testdata) as a test data. Before running TiDB lightning, ensure to extract data with the command `tar zxf in.tar.gz`.

To install Sysbench, read the official document https://github.com/akopytov/sysbench?tab=readme-ov-file#installing-from-binary-packages
