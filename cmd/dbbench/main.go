package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/sj14/dbbench/benchmark"
	"github.com/sj14/dbbench/databases"
	"github.com/spf13/pflag"
)

var (
	version = "dev version"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		// Default set of flags, available for all subcommands (benchmark options).
		defaultFlags = pflag.NewFlagSet("defaults", pflag.ExitOnError)
		iter         = defaultFlags.Int("iter", 1000, "how many iterations should be run")
		threads      = defaultFlags.Int("threads", 25, "max. number of green threads (iter >= threads > 0)")
		sleep        = defaultFlags.Duration("sleep", 0, "how long to pause after each single benchmark (valid units: ns, us, ms, s, m, h)")
		nosetup      = defaultFlags.Bool("noinit", false, "do not initialize database and tables, e.g. when only running own script")
		clean        = defaultFlags.Bool("clean", false, "only cleanup benchmark data, e.g. after a crash")
		noclean      = defaultFlags.Bool("noclean", false, "keep benchmark data")
		versionFlag  = defaultFlags.Bool("version", false, "print version information")
		runBench     = defaultFlags.String("run", "all", "only run the specified benchmarks, e.g. \"inserts deletes\"")
		scriptname   = defaultFlags.String("script", "", "custom sql file to execute")

		// Connection flags, applicable for most databases (not sqlite).
		connFlags = pflag.NewFlagSet("conn", pflag.ExitOnError)
		host      = connFlags.String("host", "`localhost`", "address of the server")
		port      = connFlags.Int("port", 0, "port of the server (0 -> db defaults)")
		user      = connFlags.String("user", "root", "user name to connect with the server")
		pass      = connFlags.String("pass", "root", "password to connect with the server")

		// Max. connections, applicable for most databases (not cassandra, sqlite).
		maxconnsFlags = pflag.NewFlagSet("conns", pflag.ExitOnError)

		// GCP specific application flags (for Spanner)

		// Flag sets for each database. DB specific flags are set in the switch statement below.
		postgresFlags = pflag.NewFlagSet("postgres", pflag.ExitOnError)
	)

	defaultFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Available subcommands:\n\tcassandra|cockroach|mssql|mysql|postgres|sqlite|spanner\n")
		fmt.Fprintf(os.Stderr, "\tUse 'subcommand --help' for all flags of the specified command.\n")
		fmt.Fprintf(os.Stderr, "Generic flags for all subcommands:\n")
		defaultFlags.PrintDefaults()
	}

	// No comamnd given. Print usage help and exit.
	if len(os.Args) < 2 {
		defaultFlags.Usage()
		os.Exit(1)
	}

	var bencher benchmark.Bencher
	switch os.Args[1] {
	case "postgres":
		postgresFlags.AddFlagSet(defaultFlags)
		postgresFlags.AddFlagSet(connFlags)
		postgresFlags.AddFlagSet(maxconnsFlags)
		if err := postgresFlags.Parse(os.Args[2:]); err != nil {
			log.Fatalf("failed to parse postgres flags: %v", err)
		}
		bencher = databases.NewPostgres(*host, *port, *user, *pass)
	case "yugabyte":
		postgresFlags.AddFlagSet(defaultFlags)
		postgresFlags.AddFlagSet(connFlags)
		postgresFlags.AddFlagSet(maxconnsFlags)
		if err := postgresFlags.Parse(os.Args[2:]); err != nil {
			log.Fatalf("failed to parse postgres flags: %v", err)
		}
		bencher = databases.NewYugabyteDB(*host, *port, *user, *pass)
	default:
		if err := defaultFlags.Parse(os.Args[1:]); err != nil {
			log.Fatalf("failed to parse default flags: %v", err)
		}

		// Only show version information and exit.
		if *versionFlag {
			fmt.Printf("dbbench %v, commit %v, built at %v\n", version, commit, date)
			os.Exit(0)
		}

		// Command not recognized. Print usage help and exit.
		defaultFlags.Usage()
		os.Exit(1)
	}

	// only clean old data when clean flag is set
	if *clean {
		bencher.Cleanup()
		fmt.Println("cleaned data")
		os.Exit(0)
	}

	// setup database
	if !*nosetup {
		bencher.Setup()
	}

	// only cleanup benchmark data when noclean flag is not set
	if !*noclean {
		defer bencher.Cleanup()
	}

	// we need at least one thread
	if *threads == 0 {
		*threads = 1
		fmt.Println("increased to 1 thread")
	}

	// can't have more threads than iterations
	if *threads > *iter {
		*threads = *iter
	}

	benchmarks := []benchmark.Benchmark{}

	// If a script was specified, overwrite built-in benchmarks.
	if *scriptname != "" {
		dat, err := ioutil.ReadFile(*scriptname)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}
		buf := bytes.NewBuffer(dat)
		benchmarks, err = benchmark.ParseScript(buf)
		if err != nil {
			log.Fatalf("failed to parse script: %v\n", err)
		}
	} else {
		// Otherwise use built-in benchmarks.
		benchmarks = bencher.Benchmarks()
	}

	// split benchmark names when "-run 'bench0 bench1 ...'" flag was used
	toRun := strings.Split(*runBench, " ")

	startTotal := time.Now()

	// notify channel for SIGINT (ctrl-c)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	for i, b := range benchmarks {
		select {
		case <-sigchan:
			// got SIGINT, stop benchmarking
			printTotal(startTotal)
			// using os.Exit(130) instead of return won't
			// run deferred funcs (e.g. b.Cleanup())
			return
		default:
			// check if we want to run this particular benchmark
			if !contains(toRun, "all") && !contains(toRun, b.Name) {
				continue
			}

			// run the particular benchmark
			results := benchmark.Run(bencher, b, *iter, *threads)

			took := results.Duration
			// execution in ns for mode once
			nsPerOp := took.Nanoseconds()

			// execution in ns/op for mode loop
			if b.Type == benchmark.TypeLoop {
				nsPerOp /= int64(*iter)
			}

			fmt.Printf(`%v (%vx) took: %v 
avg: %v, min: %v, max: %v
%v ops/s
%v ns/op

`,
				b.Name,
				results.TotalExecutionCount,
				took,
				results.Avg(),
				results.Min,
				results.Max,
				float64(results.TotalExecutionCount)/results.Duration.Seconds(),
				nsPerOp)

			// Don't sleep after the last benchmark
			if i != len(benchmarks)-1 {
				time.Sleep(*sleep)
			}
		}
	}
	printTotal(startTotal)
}

func printTotal(startTotal time.Time) {
	fmt.Printf("total: %v\n", time.Since(startTotal))
}

func contains(options []string, want string) bool {
	for _, o := range options {
		if o == want {
			return true
		}
	}
	return false
}
