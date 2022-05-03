package databases

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"time"

	"github.com/sj14/dbbench/benchmark"
)

// Postgres implements the bencher interface.
type YugabyteDB struct {
	db *pgxpool.Pool
}

// NewPostgres returns a new postgres bencher.
func NewYugabyteDB(host string, port int, user, password string) *YugabyteDB {
	if port == 0 {
		port = 5433
	}

	dataSourceName := fmt.Sprintf("postgresql://%s:%s@%s:%v/%s", user, password, host, port, "postgres")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	db, err := pgxpool.Connect(ctx, dataSourceName)
	if err != nil {
		log.Fatalf("failed to open connection: %v\n", err)
	}
	if err = db.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	p := &YugabyteDB{db: db}
	return p
}

// Benchmarks returns the individual benchmark statements for the postgres db.
func (y *YugabyteDB) Benchmarks() []benchmark.Benchmark {
	return []benchmark.Benchmark{
		{Name: "inserts", Type: benchmark.TypeLoop, Stmt: "INSERT INTO dbbench.simple (id, balance) VALUES( {{.Iter}}, {{call .RandInt63}});"},
		{Name: "selects", Type: benchmark.TypeLoop, Stmt: "SELECT * FROM dbbench.simple WHERE id = {{.Iter}};"},
		{Name: "updates", Type: benchmark.TypeLoop, Stmt: "UPDATE dbbench.simple SET balance = {{call .RandInt63}} WHERE id = {{.Iter}};"},
		{Name: "deletes", Type: benchmark.TypeLoop, Stmt: "DELETE FROM dbbench.simple WHERE id = {{.Iter}};"},
		// {"relation_insert0", benchmark.TypeLoop, "INSERT INTO dbbench.relational_one (oid, balance_one) VALUES( {{.Iter}}, {{call .RandInt63}});"},
		// {"relation_insert1", benchmark.TypeLoop, "INSERT INTO dbbench.relational_two (relation, balance_two) VALUES( {{.Iter}}, {{call .RandInt63}});"},
		// {"relation_select", benchmark.TypeLoop, "SELECT * FROM dbbench.relational_two INNER JOIN dbbench.relational_one ON relational_one.oid = relational_two.relation WHERE relation = {{.Iter}};"},
		// {"relation_delete1", benchmark.TypeLoop, "DELETE FROM dbbench.relational_two WHERE relation = {{.Iter}};"},
		// {"relation_delete0", benchmark.TypeLoop, "DELETE FROM dbbench.relational_one WHERE oid = {{.Iter}};"},RandInt63
	}
}

// Setup initializes the database for the benchmark.
func (y *YugabyteDB) Setup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	if _, err := y.db.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS dbbench"); err != nil {
		log.Fatalf("failed to create schema: %v\n", err)
	}
	if _, err := y.db.Exec(ctx, "CREATE TABLE IF NOT EXISTS dbbench.simple (id INT PRIMARY KEY, balance DECIMAL);"); err != nil {
		log.Fatalf("failed to create table: %v\n", err)
	}
	if _, err := y.db.Exec(ctx, "CREATE TABLE IF NOT EXISTS dbbench.relational_one (oid INT PRIMARY KEY, balance_one DECIMAL);"); err != nil {
		log.Fatalf("failed to create table relational_one: %v\n", err)
	}
	if _, err := y.db.Exec(ctx, "CREATE TABLE IF NOT EXISTS dbbench.relational_two (balance_two DECIMAL, relation INT PRIMARY KEY, FOREIGN KEY(relation) REFERENCES dbbench.relational_one(oid));"); err != nil {
		log.Fatalf("failed to create table relational_two: %v\n", err)
	}
}

// Cleanup removes all remaining benchmarking data.
func (y *YugabyteDB) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	if _, err := y.db.Exec(ctx, "DROP TABLE dbbench.simple"); err != nil {
		log.Printf("failed to drop table: %v\n", err)
	}
	if _, err := y.db.Exec(ctx, "DROP TABLE dbbench.relational_two"); err != nil {
		log.Printf("failed to drop table: %v\n", err)
	}
	if _, err := y.db.Exec(ctx, "DROP TABLE dbbench.relational_one"); err != nil {
		log.Printf("failed to drop table: %v\n", err)
	}

	if _, err := y.db.Exec(ctx, "DROP SCHEMA dbbench"); err != nil {
		log.Printf("failed drop schema: %v\n", err)
	}
	y.db.Close()
}

// Exec executes the given statement on the database.
func (y *YugabyteDB) Exec(stmt string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	_, err := y.db.Exec(ctx, stmt)
	if err != nil {
		log.Printf("%v failed: %v", stmt, err)
	}
}
