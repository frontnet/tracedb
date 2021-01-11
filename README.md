# unitdb [![GoDoc](https://godoc.org/github.com/unit-io/unitdb?status.svg)](https://pkg.go.dev/github.com/unit-io/unitdb) [![Go Report Card](https://goreportcard.com/badge/github.com/unit-io/unitdb)](https://goreportcard.com/report/github.com/unit-io/unitdb) [![Build Status](https://travis-ci.org/unit-io/unitdb.svg?branch=master)](https://travis-ci.org/unit-io/unitdb) [![Coverage Status](https://coveralls.io/repos/github/unit-io/unitdb/badge.svg?branch=master)](https://coveralls.io/github/unit-io/unitdb?branch=master)

Unitdb is blazing fast specialized time-series database for microservices, IoT, and realtime internet connected devices. The unitdb satisfy the requirements for low latency and binary messaging, it is a perfect time-series database for applications such as internet of things and internet connected devices.

```
Don't forget to ⭐ this repo if you like Unitdb!
```

# About unitdb 

## Key characteristics
- 100% Go
- Can store larger-than-memory data sets
- Optimized for fast lookups and writes
- Supports writing billions of messages (or metrics) per hour
- Supports opening database with immutable flag
- Supports database encryption
- Supports time-to-live on message entries
- Supports writing to wildcard topics
- Data is safely written to disk with accuracy and high performant block sync technique

## Quick Start
To build Unitdb from source code use go get command.

> go get -u github.com/unit-io/unitdb

## Usage
Detailed API documentation is available using the [go.dev](https://pkg.go.dev/github.com/unit-io/unitdb) service.

Make use of the client by importing it in your Go client source code. For example,

> import "github.com/unit-io/unitdb"

The in-memory key-value data store persist entries into a WAL for immediate durability. The Write Ahead Log (WAL) retains memdb data when the db restarts. The WAL ensures data is durable in case of an unexpected failure. Make use of the client by importing in your Go client source code. For example,

> import "github.com/unit-io/unitdb/memdb"

Unitdb supports Get, Put, Delete operations. It also supports encryption, batch operations, and writing to wildcard topics. See [usage guide](https://github.com/unit-io/unitdb/tree/master/docs/usage.md). 

Samples are available in the cmd directory for reference.

## Projects Using Unitdb
Below is a list of projects that use Unitdb.

- [unite](https://github.com/unit-io/unite) Lightweight, high performance messaging system for microservices, and internet connected devices.

## Architecture Overview
The unitdb engine handles data from the point put request is received through writing data to the physical disk. Data is compressed and encrypted (if encryption is set) then written to a WAL for immediate durability. Entries are written to memdb and become immediately queryable. The memdb entries are periodically written to log files in the form of blocks.

To efficiently compact and store data, the unitdb engine groups entries sequence by topic key, and then orders those sequences by time and each block keep offset of previous block in reverse time order. Index block offset is calculated from entry sequence in the time-window block. Data is read from data block using index entry information and then it un-compresses the data on read (if encryption flag was set then it un-encrypts the data on read).

<p align="left">
  <img src="docs/img/architecture-overview.png" />
</p>

Unitdb stores compressed data (live records) in a memdb store. Data records in a memdb store are partitioned into (live) time-blocks of configured capacity. New time-blocks are created at ingestion, while old time-blocks are appended to the log files and later sync to the disk block store.

When Unitdb receives an upsert records for ingestion, it first writes upsert records into memdb tiny-logs for recovery. Tiny-logs are added to the log queue to write the records to log files. The tiny-log write is triggered byte the time or size of tiny-log incase of backoff due to massive loads. 

The tiny-log queue is maintained in memory with a pre-configured size, and during massive loads the memdb backoff process will block the ingestion from proceeding before the tiny-log queue is cleared by a write operation. After upsert records are appended to the memdb tiny-logs and written to the log files the records are sync to the disk block store.

<p align="left">
  <img src="docs/img/memdb-upsert.png" />
</p>

## Next steps
In the future, we intend to enhance the Unitdb with the following features:

- Distributed design: We are working on building out the distributed design of Unitdb, including replication and sharding management to improve its scalability.
- Developer support and tooling: We are working on building more intuitive tooling, refactoring code structures, and enriching documentation to improve the onboarding experience, enabling developers to quickly integrate Unitdb to their time-series database stack.
- Expanding feature set: We also plan to expand our query feature set to include functionality such as window functions and nested loop joins.
- Query engine optimization: We will also be looking into developing more advanced ways to optimize query performance such as GPU memory caching.

## Contributing
As Unitdb is under active development and at this time Unitdb is not seeking major changes or new features from new contributors. However, small bugfixes are encouraged.

## Licensing
This project is licensed under [Apache-2.0 License](https://github.com/unit-io/unitdb/blob/master/LICENSE).
