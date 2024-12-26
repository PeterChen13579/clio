# <img src='./docs/img/xrpl-logo.svg' width='40' valign="top" /> Clio

[![Build status](https://github.com/XRPLF/clio/actions/workflows/build.yml/badge.svg?branch=develop)](https://github.com/XRPLF/clio/actions/workflows/build.yml?query=branch%3Adevelop)
[![Nightly release status](https://github.com/XRPLF/clio/actions/workflows/nightly.yml/badge.svg?branch=develop)](https://github.com/XRPLF/clio/actions/workflows/nightly.yml?query=branch%3Adevelop)
[![Clang-tidy checks status](https://github.com/XRPLF/clio/actions/workflows/clang-tidy.yml/badge.svg?branch=develop)](https://github.com/XRPLF/clio/actions/workflows/clang-tidy.yml?query=branch%3Adevelop)
[![Code coverage develop branch](https://codecov.io/gh/XRPLF/clio/branch/develop/graph/badge.svg?)](https://app.codecov.io/gh/XRPLF/clio)

Clio is an XRP Ledger API server optimized for RPC calls over WebSocket or JSON-RPC.
It stores validated historical ledger and transaction data in a more space efficient format, and uses up to 4 times less space than [rippled](https://github.com/XRPLF/rippled).

Clio can be configured to store data in [Apache Cassandra](https://cassandra.apache.org/_/index.html) or [ScyllaDB](https://www.scylladb.com/), enabling scalable read throughput.
Multiple Clio nodes can share access to the same dataset, which allows for a highly available cluster of Clio nodes without the need for redundant data storage or computation.

## 📡 Clio and `rippled`

Clio offers the full `rippled` API, with the caveat that Clio by default only returns validated data. This means that `ledger_index` defaults to `validated` instead of `current` for all requests. Other non-validated data, such as information about queued transactions, is also not returned.

Clio retrieves data from a designated group of `rippled` nodes instead of connecting to the peer-to-peer network.
For requests that require access to the peer-to-peer network, such as `fee` or `submit`, Clio automatically forwards the request to a `rippled` node and propagates the response back to the client. To access non-validated data for *any* request, simply add `ledger_index: "current"` to the request, and Clio will forward the request to `rippled`.

> [!NOTE]  
> Clio requires access to at least one `rippled` node, which can run on the same machine as Clio or separately.

## 📚 Learn more about Clio

Below are some useful docs to learn more about Clio.

**For Developers**:

- [How to build Clio](./docs/build-clio.md)
- [Metrics and static analysis](./docs/metrics-and-static-analysis.md)
- [Coverage report](./docs/coverage-report.md)

**For Operators**:

- [How to configure Clio and rippled](./docs/configure-clio.md)
- [How to run Clio](./docs/run-clio.md)
- [Logging](./docs/logging.md)
- [Troubleshooting guide](./docs/trouble_shooting.md)

**General reference material:**

- [API reference](https://xrpl.org/http-websocket-apis.html)
- [Developer docs](https://xrplf.github.io/clio)
- [Clio documentation](https://xrpl.org/the-clio-server.html#the-clio-server)

## 🆘 Help

Feel free to open an [issue](https://github.com/XRPLF/clio/issues) if you have a feature request or something doesn't work as expected.
If you have any questions about building, running, contributing, using Clio or any other, you could always start a new [discussion](https://github.com/XRPLF/clio/discussions).

##  Regarding this branch
Clio stores the ledger in a flat format in the database, which is more efficient to query. But it also brings the risk of data corruption. Data corruption can happen due to various reasons, like hardware failure, database migration issue or Clio's bug. 
This branch will build a data validation tool which can check the integrity of Clio's database cryptographically.
The tool is located in "tools/db_checker" after building Clio normally.
It can check ledger states and transactions, also it can check other indexing tables Clio is using, like account_tx, ledger_hashes.
The tool is developed in go, **please install go before building it**.

There are two kinds of data to check, states and transactions.

### States
States are the ledger entities that are stored in the **objects** / **successor** / **diff** tables.
To verify the states for one ledger, we need to traverse the linked list formed by the **successor** table and add the actual entities binary (from **objects**) to SHAMap to verify the integrity of the ledger states.

*Be aware the states check can overwhelm database, using --cursors to adjust the concurrent database requests*

To check the states for ledger SEQ, you can use the following command:
```
./tools/db_checker/db_checker --objects --keyspace clio --fromLedgerIdx SEQ  --toLedgerIdx SEQ 127.0.0.1

```

The **diff** table stores the changed entities indexes of the ledger, After we have verified a valid ledger, we can add the changed entities to SHAMap to verify the states incrementally. But this will skip the **successor** table validation.
To check the states incrementally, you can use the following command:
```
./tools/db_checker/db_checker --diff --keyspace clio --fromLedgerIdx SEQ  --toLedgerIdx OTHER_SEQ 127.0.0.1
```

### Transactions
Clio stores all transactions metadata in the **transactions** table. To verify the transactions for one ledger cryptographically, we need to find all the transactions of the ledger in ledger_transactions table and add the transactions binary to SHAMap. After verifying the **transactions** and **ledger_transactions** table, we can start to verify other indexing tables, such as nf_tokens, account_tx.

*Be aware the transactions check is much faster than states check, it also requires less bandwidth of database. This tool can do multiple transactions check concurrently, using --step to specify the number of transactions check concurrently*

To check the transactions for ledger SEQ, you can use the following command:
```
./tools/db_checker/db_checker --tx --keyspace clio --fromLedgerIdx SEQ  --toLedgerIdx SEQ 127.0.0.1
```
To check the transactions in range [SEQ1, SEQ2], you can use the following command:
```
./tools/db_checker/db_checker --tx --keyspace clio --fromLedgerIdx SEQ1  --toLedgerIdx SEQ2 --step 2 127.0.0.1
```
Also the crptographic check can be skipped by adding --txSkipSha to the command.
```
./tools/db_checker/db_checker --tx --txSkipSha --keyspace clio --fromLedgerIdx SEQ1  --toLedgerIdx SEQ2 --step 2 127.0.0.1
```
By adding --txSkipNFT to the command, the tool will skip checking the nft related table, including **nf_tokens**, **issuer_nf_tokens_v2**, **nf_token_uris** and **nf_token_transactions**.

By adding --txSkipAccount to the command, the tool will skip checking the account related table, including **account_tx**.

To check the **ledger_hashes** table, you can use the following command:
```
./tools/db_checker/db_checker --ledgerHash --keyspace clio --fromLedgerIdx SEQ1  --toLedgerIdx SEQ2 --step 2
```

### Fix corrupted data
When corrupted data is found, you can see error in the log.

This tool provides a way to fix the corrupted data of **ledger_hashes** table and **nf_token_uris** table in place. Please take these two as examples, if you want to fix other indexing tables.

To fix the corrupted data of **ledger_hashes** table, you can use the following command:
```
./tools/db_checker/db_checker --ledgerHashFix --keyspace clio --fromLedgerIdx SEQ1  --toLedgerIdx SEQ2 --step 2 127.0.0.1
```

To fix the corrupted data of **nf_token_uris** table, you can use the following command:
```
./tools/db_checker/db_checker --tx --NFTUriFix --keyspace clio --fromLedgerIdx SEQ1  --toLedgerIdx SEQ2 --step 2 127.0.0.1

```
If the corruption happens to the non-indexing table unfortunately, we have to obtain the correct data from other trust sources. 
*fix_transaction.py* provides an example to fix the corrupted **transactions** by obtaining the data from xrplcluster rippled.

It is worth noting that this tool can work with the cassandra_delete_range tool to refill all the data from a certain ledger. Typically, when the successor table is corrupted,
we can use this tool to find the newest valid ledger and use cassandra_delete_range to delete all the data older than the valid ledger, then Clio will refill the data from the valid ledger.

---
This tool also provides parameters to control the database access,eg auth, timeout and the number of concurrent database requests.
Please see the help message for more details.

Please refer to the shell scripts in the tools/db_checker folder for more examples[^1].
| file  | usage  | detail  |
|---|---|---|
| run_diff_check.sh  | nohup ../tools/db_checker/run_fix_nft_uri.sh db_checker_path 84261373  | check states and diff from 84261373 forward  |
| run_fix_ledgerhash.sh  | nohup ../tools/db_checker/run_fix_nft_uri.sh db_checker_path 84261373  | fix ledger hashes from 84261373 forward  |
| run_state_check.sh | nohup ../tools/db_checker/run_state_check.sh db_checker_path 84261373  | check states from 84261373 forward  |
| run_tx_check.sh | nohup ../tools/db_checker/run_tx_check.sh db_checker_path 84261373  | check tx from 84261373 forward  |
| run_fix_nft_uri.sh | nohup ../tools/db_checker/run_fix_nft_uri.sh db_checker_path 84261373  | check tx from 84261373 forward and fix nf_token_uris if anything missing  |

[^1]: Please fill all nessary parameters in the shell scripts before running them.


