# dbbench

![Action](https://github.com/sj14/dbbench/workflows/Go/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/sj14/dbbench)](https://goreportcard.com/report/github.com/sj14/dbbench)
[![Coverage Status](https://coveralls.io/repos/github/sj14/dbbench/badge.svg?branch=master)](https://coveralls.io/github/sj14/dbbench?branch=master)

## Table of Contents

- [Description](#Description)
- [Example](#example)
- [Installation](#installation)
- [Supported Databases](#Supported-Databases-/-Driver)
- [Usage](#usage)
- [Custom Scripts](#custom-scripts)
- [Troubeshooting](#troubleshooting)
- [Development](#development)
- [Acknowledgements](#Acknowledgements)

## Description

`dbbench` is a simple tool to benchmark or stress test databases. You can use the simple built-in benchmarks or run your own queries.  

**Attention**: This tool comes with no warranty. Don't run it on production databases.

## Example

``` text
$ dbbench postgres --user postgres --pass example --iter 100000
inserts 6.199670776s    61996   ns/op
updates 7.74049898s     77404   ns/op
selects 2.911541197s    29115   ns/op
deletes 5.999572479s    59995   ns/op
total: 22.85141994s
```

## Installation

### Precompiled Binaries

Binaries are available for all major platforms. See the [releases](https://github.com/sj14/dbbench/releases) page. Unfortunately, `cgo` is disabled for these builds, which means there is *no SQLite support* ([#1](https://github.com/sj14/dbbench/issues/1)).

### Homebrew

Using the [Homebrew](https://brew.sh/) package manager for macOS:

``` text
brew install sj14/tap/dbbench
```

### Manually

It's also possible to install the current development snapshot with `go get` (not recommended):

``` text
go get -u github.com/sj14/dbbench/cmd/dbbench
```

## Supported Databases / Driver

Databases | Driver
----------|-----------
PostgreSQL and compatible databases (e.g. YugabyteDB) | github.com/jackc/pgx


## Usage

``` text
Available subcommands:
        postgres|yugabyte
        Use 'subcommand --help' for all flags of the specified command.
Generic flags for all subcommands:
      --clean            only cleanup benchmark data, e.g. after a crash
      --iter int         how many iterations should be run (default 1000)
      --noclean          keep benchmark data
      --noinit           do not initialize database and tables, e.g. when only running own script
      --run string       only run the specified benchmarks, e.g. "inserts deletes" (default "all")
      --script string    custom sql file to execute
      --sleep duration   how long to pause after each single benchmark (valid units: ns, us, ms, s, m, h)
      --threads int      max. number of green threads (iter >= threads > 0) (default 25)
      --version          print version information
```

## Custom Scripts

You can run your own SQL statements with the `--script` flag. You can use the auto-generate tables. Beware the file size as it will be completely loaded into memory.

The script must contain valid SQL statements for your database.

There are some built-in variables and functions which can be used in the script. It's using the golang [template engine](https://golang.org/pkg/text/template/) which uses the delimiters `{{` and `}}`. Functions are executed with the `call` command and arguments are passed after the function name.

### Benchmark Settings

A new benchmark is created with the `\benchmark` keyword, followed by either `once` or `loop`. Optional parameters can be added afterwards in the same line.

The the usage description and the example subsection for more information.

Usage                     | Description                                   |
--------------------------|-----------------------------------------------|
`\benchmark once`                | Execute the following statements (lines) only once (e.g. to create and delete tables).
`\benchmark loop`                | Default mode. Execute the following statements (lines) in a loop. Executes them one after another and then starts a new iteration. Add another `\benchmark loop` to start another benchmark of statements.
`\name insert`              | Set a custom name for the DB statement(s), which will be output instead the line numbers (`insert` is an examplay name).

### Statement Substitutions

Usage                     | Description                                   |
--------------------------|-----------------------------------------------|
`{{.Iter}}`                 | The iteration counter. Will return `1` when `\benchmark once`.
`{{call .Seed 42}}`         | [godoc](https://golang.org/pkg/math/rand/#Seed) (`42` is an examplary seed)
`{{call .RandInt63}}`       | [godoc](https://golang.org/pkg/math/rand/#Int63)
`{{call .RandInt63n 9999}}` | [godoc](https://golang.org/pkg/math/rand/#Int63n) (`9999` is an examplary upper limit)
`{{call .RandFloat32}}`     | [godoc](https://golang.org/pkg/math/rand/#Float32)  
`{{call .RandFloat64}}`     | [godoc](https://golang.org/pkg/math/rand/#Float64)
`{{call .RandExpFloat64}}`  | [godoc](https://golang.org/pkg/math/rand/#ExpFloat64)
`{{call .RandNormFloat64}}` | [godoc](https://golang.org/pkg/math/rand/#NormFloat64)

### Example

Exemplary `sqlite_bench.sql` file:

``` sql
-- Create table
\benchmark once \name init
CREATE TABLE dbbench_simple (id INT PRIMARY KEY, balance DECIMAL);

-- How long takes an insert and delete?
\benchmark loop \name single
INSERT INTO dbbench_simple (id, balance) VALUES({{.Iter}}, {{call .RandInt63}});
DELETE FROM dbbench_simple WHERE id = {{.Iter}}; 

-- How long takes it in a single transaction?
\benchmark loop \name batch
BEGIN TRANSACTION;
INSERT INTO dbbench_simple (id, balance) VALUES({{.Iter}}, {{call .RandInt63}});
DELETE FROM dbbench_simple WHERE id = {{.Iter}}; 
COMMIT;

-- Delete table
\benchmark once \name clean
DROP TABLE dbbench_simple;
```

In this script, we create and delete the table manually, thus we will pass the `--noinit` and `--noclean` flag, which would otherwise create this default table for us:

``` text
dbbench postgres --script scripts/sqlite_bench.sql --iter 5000 --noinit --noclean
```

output:

``` text
(once) init:    3.404784ms      3404784 ns/op
(loop) single:  10.568390874s   2113678 ns/op
(loop) batch:   5.739021596s    1147804 ns/op
(once) clean:   1.065703ms      1065703 ns/op
total: 16.312319959s
```

## Troubleshooting

**Error message**

``` text
failed to insert: UNIQUE constraint failed: dbbench_simple.id
```

**Description**
The previous data wasn't removed (e.g. because the benchmark was canceled). Try to run the same command again, but with the `--clean` flag attached, which will remove the old data. Then run the original command again.

---


## Development

Below are some examples how to run different databases and the equivalent call of `dbbench` for testing/developing.

### PostgreSQL

``` text
docker run --name dbbench-postgres -p 5432:5432 -d postgres
```

``` text
dbbench postgres --user postgres --pass example
```

### YugabyteDB

``` text
docker pull yugabytedb/yugabyte:2.13.1.0-b112
docker run -d --name yugabyte  -p7000:7000 -p9000:9000 -p5433:5433 -p9042:9042 yugabytedb/yugabyte:2.13.1.0-b112 bin/yugabyted start --daemon=false --ui=false
```

``` text
dbbench yugabyte
```

## Acknowledgements

Thanks to the authors of Go and those of the directly and indirectly used libraries, especially the driver developers. It wouldn't be possible without all your work.

This tool was highly inspired by the snippet from user [Fale](https://github.com/cockroachdb/cockroach/issues/23061#issue-300012178) and the tool [pgbench](https://www.postgresql.org/docs/current/pgbench.html). Later, also inspired by [MemSQL's dbbench](https://github.com/memsql/dbbench) which had the name and a similar idea before.