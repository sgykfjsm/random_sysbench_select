package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
)

const (
	sbtest = "sbtest"
)

var (
	columns = []string{"id", "k", "c", "pad"} // columns of the table "sbtest"
)

// table structure of sbtest
//
// CREATE TABLE `sbtest1` (
// 	`id` int(11) NOT NULL AUTO_INCREMENT,
// 	`k` int(11) NOT NULL DEFAULT '0',
// 	`c` char(120) NOT NULL DEFAULT '',
// 	`pad` char(60) NOT NULL DEFAULT '',
// 	PRIMARY KEY (`id`) /*T![clustered_index] CLUSTERED */,
// 	KEY `k_1` (`k`)
//   ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin AUTO_INCREMENT=30001;
//

type Job struct {
	ID        int
	DbName    string
	TableName string
	Fields    []string
}

type Result struct {
	JobID  int
	Elapse time.Duration
	OK     bool
	Err    error
}

type Report struct {
	JobStartAt        time.Time      `json:"started_at" yaml:"started_at"`
	JobEndAt          time.Time      `json:"ended_at" yaml:"ended_at"`
	TotalQueries      int            `json:"total_queries" yaml:"total_queries"`
	TotalSuccessCount int            `json:"total_success" yaml:"total_success"`
	TotalFailureCount int            `json:"total_failure" yaml:"total_failure"`
	SuccessRate       float64        `json:"success_rate" yaml:"success_rate"`
	FailureRate       float64        `json:"error_rate" yaml:"error_rate"`
	QPS               float64        `json:"qps" yaml:"qps"`
	QueryLatencyP99   float64        `json:"query_latency_p95" yaml:"query_latency_p95"`
	QueryLatencyP95   float64        `json:"query_latency_p90" yaml:"query_latency_p90"`
	QueryLatencyP80   float64        `json:"query_latency_p80" yaml:"query_latency_p80"`
	QueryLatencyP50   float64        `json:"query_latency_p50" yaml:"query_latency_p50"`
	CustomData        map[string]any `json:"custom_data,omitempty" yaml:"custom_data,omitempty"`

	totalQueryDurations []time.Duration
}

func (rep *Report) AppendQueryDuration(queryDuration time.Duration) {
	rep.totalQueryDurations = append(rep.totalQueryDurations, queryDuration)
}

func p99(idx int) int {
	return int(0.99 * float64(idx))
}

func p95(idx int) int {
	return int(0.95 * float64(idx))
}

func p80(idx int) int {
	return int(0.80 * float64(idx))
}

func p50(idx int) int {
	return int(0.50 * float64(idx))
}

func (rep *Report) calculateQueryDurationPercentile() {
	sort.Slice(rep.totalQueryDurations, func(i, j int) bool {
		return rep.totalQueryDurations[i] > rep.totalQueryDurations[j]
	})

	idxNum := len(rep.totalQueryDurations)
	rep.QueryLatencyP99 = float64(rep.totalQueryDurations[p99(idxNum)])
	rep.QueryLatencyP95 = float64(rep.totalQueryDurations[p95(idxNum)])
	rep.QueryLatencyP80 = float64(rep.totalQueryDurations[p80(idxNum)])
	rep.QueryLatencyP50 = float64(rep.totalQueryDurations[p50(idxNum)])
}

func (rep *Report) finalizeReport() {
	rep.calculateQueryDurationPercentile()

	rep.TotalQueries = rep.TotalSuccessCount + rep.TotalFailureCount
	rep.SuccessRate = float64(rep.TotalSuccessCount) / float64(rep.TotalQueries)
	rep.FailureRate = 1.00 - rep.SuccessRate
	rep.QPS = float64(rep.TotalQueries) / rep.JobEndAt.Sub(rep.JobStartAt).Seconds()
}

func (rep *Report) PrintResult(format string) {
	rep.finalizeReport()
	switch strings.ToLower(format) {
	case "json":
		rep.PrintResultJson()
	case "yaml":
		rep.PrintResultYaml()
	case "table":
		rep.PrintResultTable()
	default:
		// Noop
	}
}

func (rep *Report) PrintResultJson() {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Println("Error: failed to print json:", err)
		return
	}

	fmt.Println(string(data))
}

func (rep *Report) PrintResultYaml() {
	data, err := yaml.Marshal(rep)
	if err != nil {
		fmt.Println("Error: failed to print yaml:", err)
		return
	}

	fmt.Println(string(data))
}

func (rep *Report) PrintResultTable() {
	fmt.Printf("Job started at %v\n", rep.JobStartAt)
	fmt.Printf("Job ended at %v\n", rep.JobEndAt)
	fmt.Printf("Total Queries:    %d\n", rep.TotalQueries)
	fmt.Printf("Total Success:    %d\n", rep.TotalSuccessCount)
	fmt.Printf("Total Failure:    %d\n", rep.TotalFailureCount)
	fmt.Printf("Success rate:     %.2f\n", rep.SuccessRate)
	fmt.Printf("Failure rate:     %.2f\n", rep.FailureRate)
	fmt.Printf("QPS:              %.2f\n", rep.QPS)
	fmt.Printf("Latency P99 (ms): %.2f\n", rep.QueryLatencyP99)
	fmt.Printf("Latency P95 (ms): %.2f\n", rep.QueryLatencyP95)
	fmt.Printf("Latency P80 (ms): %.2f\n", rep.QueryLatencyP80)
	fmt.Printf("Latency P50 (ms): %.2f\n", rep.QueryLatencyP50)
}

func getDbNameRandom(r *rand.Rand, dbNumMax int) string {
	return fmt.Sprintf("%s%03d", sbtest, r.IntN(dbNumMax)+1)
}

func getTableNameRandom(r *rand.Rand, tableNumMax int) string {
	return fmt.Sprintf("%s%d", sbtest, r.IntN(tableNumMax)+1)
}

func getTableFieldsRandom(r *rand.Rand) []string {
	if r.Float64() < 0.1 {
		return []string{"*"}
	}

	fieldNum := r.IntN(len(columns)) + 1

	idx := r.Perm(len(columns))[:fieldNum]
	var fields []string
	for _, i := range idx {
		fields = append(fields, columns[i])
	}

	return fields
}

func pickUpCSVFile(tableName, csvDir string) (string, error) {
	// target CSV file name is like "sbtest001.${table_name}.0000000010000.csv"
	targetFileNamePattern := fmt.Sprintf("sbtest001.%s.*.csv", tableName)
	files, err := filepath.Glob(filepath.Join(csvDir, targetFileNamePattern))
	if err != nil {
		return "", err
	} else if len(files) == 0 {
		return "", fmt.Errorf("not found the CSV file for the table %s in %s", tableName, csvDir)
	}

	return files[0], nil
}

func loadCSVDataRandom(r *rand.Rand, tableName, csvDir string) ([]string, error) {
	csvFile, err := pickUpCSVFile(tableName, csvDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find CSV file: %w", err)
	}

	f, err := os.Open(csvFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", csvFile, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	var data []string
	for counter := 1; ; counter++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to read CSV file %s: %w", csvFile, err)
		}

		if r.IntN(counter) == 0 {
			data = row
		}
	}

	return data, nil
}

func unloadCSVDataRandomFromCache(r *rand.Rand, tableName string) ([]string, error) {
	if rows, ok := csvCache[tableName]; ok {
		return rows[r.IntN(len(rows))], nil
	}

	return nil, fmt.Errorf("CSV data for the table %s is not found in cache", tableName)
}

func LogErrAndExit(msg string, err error, code int) {
	slog.Error(msg, slog.String("error", err.Error()))
	os.Exit(code)
}

func setupDB(addr, user, password string, n int) (*sql.DB, error) {
	cfg := mysql.NewConfig()
	cfg.Addr = addr
	cfg.User = user
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.ParseTime = true

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n) // recommended to be set same to db.SetMaxOpenConns()

	return db, nil
}

func main() {
	// DB setting
	dbAddress := flag.String("addr", "127.0.0.1:4000", "address to access the database server")
	dbUser := flag.String("user", "root", "db user name")
	dbPassword := flag.String("password", "", "password to access the database server")
	csvDir := flag.String("csv", "in", "path to CSV directory")
	dbNumMax := flag.Int("dbnum", 50, "upper limit number of the database")
	tableNumMax := flag.Int("tablenum", 1000, "upper limit number of the table in a single database")

	// client setting
	duration := flag.Int("duration", 10, "time of second to keep running the process")
	workers := flag.Int("worker", 10, "number of workers to run the job")
	qps := flag.Int("qps", 1000, "expected QPS to execute the query")

	// misc
	format := flag.String("format", "json", "format of printing the job result. Support json, yaml and table")
	isDebug := flag.Bool("debug", false, "if true, a logging level becomes DEBUG")

	flag.Parse()

	if *isDebug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	r := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64()))
	durationSecond := time.Duration(*duration) * time.Second

	slog.Info("loading CSV file list")
	if err := loadCSVDataToCache(*csvDir); err != nil {
		LogErrAndExit("failed to load CSV file list to cache", err, 1)
	}
	slog.Info("saved CSV data into cache", slog.Int("count", len(csvCache)))

	slog.Info("prepare the database tasks")
	db, err := setupDB(*dbAddress, *dbUser, *dbPassword, *workers)
	if err != nil {
		LogErrAndExit(
			fmt.Sprintf("failed to connect the database %s by the user %s", *dbAddress, *dbUser), err, 1)
	}
	defer db.Close()
	slog.Info("opened the database connection")

	slog.Info("prepare job workers")
	jobs := make(chan Job, *qps*2) // job buffer
	result := make(chan Result, *qps*2)

	var wg sync.WaitGroup
	wg.Add(*workers)
	for i := 1; i <= *workers; i++ {
		go run(r, db, jobs, result, &wg)
	}
	slog.Info(fmt.Sprintf("launching %d workers", *workers))

	report := &Report{}
	var reportWg sync.WaitGroup
	reportWg.Add(1)
	go func() {
		defer reportWg.Done()

		for res := range result {
			if res.OK {
				report.TotalSuccessCount += 1
				report.AppendQueryDuration(res.Elapse)
			} else {
				report.TotalFailureCount += 1
			}
			if res.Err != nil && r.Float64() < 0.1 {
				slog.Debug("something wrong", slog.String("err", res.Err.Error()))
			}
		}
	}()
	slog.Info("finished to prepare job workers")

	endJobAt := time.Now().Add(durationSecond)
	slog.Info("set QPS as a rate limit", slog.Int("qps", *qps))
	limiter := rate.NewLimiter(rate.Limit(*qps), *qps)

	slog.Info(fmt.Sprintf("start the job for %d seconds", *duration))
	report.JobStartAt = time.Now()
	for jobID := 1; time.Now().Before(endJobAt); jobID++ {
		// blocking the routine and waiting for token in order to honor the limit
		if err := limiter.Wait(context.Background()); err != nil {
			slog.Error("failed to acquire token", slog.String("error", err.Error()))
			continue
		}
		jobs <- Job{
			ID:        jobID,
			DbName:    getDbNameRandom(r, *dbNumMax),
			TableName: getTableNameRandom(r, *tableNumMax),
			Fields:    getTableFieldsRandom(r),
		}
	}
	report.JobEndAt = time.Now()

	slog.Info("waiting for all job completion")
	close(jobs)
	wg.Wait()

	close(result)
	reportWg.Wait()

	report.PrintResult(*format)
	slog.Info("good bye")
}

var csvCache map[string][][]string

func loadCSVDataToCache(csvDir string) error {
	// initialize cache
	csvCache = make(map[string][][]string)

	// expected CSV file name is like "sbtest001.sbtest1.0000000010000.csv"
	targetFileNamePattern := "sbtest001.*.0000000010000.csv"
	files, err := filepath.Glob(filepath.Join(csvDir, targetFileNamePattern))
	if err != nil {
		return err
	}

	for _, file := range files {
		tableName, rows, err := readAllCSV(file)
		if err != nil {
			return err
		}
		csvCache[tableName] = rows
	}

	return nil
}

func readAllCSV(file string) (string, [][]string, error) {
	// expected CSV file name is like "sbtest001.sbtest1.0000000010000.csv"
	// tableName should be like "sbtest1"
	tableName := strings.Split(filepath.Base(file), ".")[1]
	f, err := os.Open(file)
	if err != nil {
		return "", nil, err
	}

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return "", nil, err
	}

	return tableName, rows, nil
}

func run(r *rand.Rand, db *sql.DB, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		result := Result{JobID: job.ID}

		data, err := unloadCSVDataRandomFromCache(r, job.TableName)
		if err == nil {
			whereClause := generateWhereClauseRandom(r, data)
			selectQuery := buildRandomSelectQuery(r, job.DbName, job.TableName, job.Fields, whereClause)

			slog.Debug("build a select query", slog.String("query", selectQuery))
			start := time.Now()
			if _, err := db.Exec(selectQuery); err == nil {
				result.OK = true
				result.Elapse = time.Duration(time.Since(start).Microseconds())
			} else {
				// something goes wrong
				result.Err = err
			}
		} else {
			result.Err = err
		}

		results <- result
	}
}

func buildRandomSelectQuery(r *rand.Rand, dbName, tableName string, fields []string, whereClause string) string {
	var builder strings.Builder
	builder.Grow(256)

	builder.WriteString("SELECT ")
	builder.WriteString(strings.Join(fields, ", "))
	builder.WriteString(" FROM ")
	builder.WriteString(dbName)
	builder.WriteString(".")
	builder.WriteString(tableName)
	builder.WriteString(" ")
	builder.WriteString(whereClause)

	if r.Float64() < 0.1 {
		builder.WriteString(" ORDER BY ")
		if fields[0] == "*" {
			builder.WriteString("id")
		} else {
			builder.WriteString(fields[0])
		}
	}
	builder.WriteString(";")

	return builder.String()
}

func generateWhereClauseRandom(r *rand.Rand, data []string) string {
	n := r.IntN(len(data)) + 1

	clause := make([]string, 0, n)

	for idx := range r.Perm(len(data))[:n] {
		if r.Float64() < 0.1 {
			data[idx] = "-1" // intentional invalid condition value
		}

		var builder strings.Builder
		builder.Grow(128)
		builder.WriteString(columns[idx])
		builder.WriteString(" = ")

		switch idx {
		case 0, 1: // id, k
			builder.WriteString(data[idx])
		case 2, 3: // c, pad
			builder.WriteString("'")
			builder.WriteString(data[idx])
			builder.WriteString("'")
		}

		clause = append(clause, builder.String())
	}

	return "WHERE " + strings.Join(clause, " AND ")
}
