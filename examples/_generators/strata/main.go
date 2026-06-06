// Command strata-gen creates examples/strata/sample.db, a small SQLite database
// for exploring with runtime-strata. It uses the SAME pure-Go driver the app
// uses (modernc.org/sqlite), so no cgo or sqlite3 CLI is required.
//
// Run from the repo root:
//
//	go run ./examples/_generators/strata
//
// Then explore it with:
//
//	./bin/runtime-strata "sqlite:file:examples/strata/sample.db"
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	out := "examples/strata/sample.db"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fail(err)
	}
	// Start clean so re-running is deterministic.
	_ = os.Remove(out)

	db, err := sql.Open("sqlite", "file:"+out)
	if err != nil {
		fail(err)
	}
	defer db.Close()

	if err := seed(db); err != nil {
		fail(err)
	}

	fmt.Printf("created %s (tables: employees, departments, sales)\n", out)
}

func seed(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE departments (
			id          INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			location    TEXT NOT NULL
		)`,
		`CREATE TABLE employees (
			id          INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			email       TEXT NOT NULL,
			dept_id     INTEGER NOT NULL REFERENCES departments(id),
			salary      INTEGER NOT NULL,
			start_date  TEXT NOT NULL,
			active      INTEGER NOT NULL
		)`,
		`CREATE TABLE sales (
			order_id    INTEGER PRIMARY KEY,
			order_date  TEXT NOT NULL,
			product     TEXT NOT NULL,
			quantity    INTEGER NOT NULL,
			net_total   REAL NOT NULL,
			region      TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}

	depts := [][]any{
		{1, "Engineering", "Berlin"},
		{2, "Research", "Cambridge"},
		{3, "Platform", "Austin"},
		{4, "Networking", "Tokyo"},
	}
	for _, d := range depts {
		if _, err := db.Exec(`INSERT INTO departments(id,name,location) VALUES(?,?,?)`, d...); err != nil {
			return fmt.Errorf("departments: %w", err)
		}
	}

	emps := [][]any{
		{1, "Ada Lovelace", "ada.lovelace@example.com", 1, 142000, "2019-03-11", 1},
		{2, "Alan Turing", "alan.turing@example.com", 2, 155000, "2017-06-23", 1},
		{3, "Grace Hopper", "grace.hopper@example.com", 1, 138500, "2018-01-15", 1},
		{4, "Katherine Johnson", "katherine.johnson@example.com", 2, 131000, "2020-09-01", 1},
		{5, "Margaret Hamilton", "margaret.hamilton@example.com", 1, 147250, "2016-11-30", 1},
		{6, "Dennis Ritchie", "dennis.ritchie@example.com", 3, 151000, "2015-04-19", 0},
		{7, "Barbara Liskov", "barbara.liskov@example.com", 2, 149900, "2019-08-05", 1},
		{8, "Linus Torvalds", "linus.torvalds@example.com", 3, 160000, "2014-02-28", 1},
		{9, "Radia Perlman", "radia.perlman@example.com", 4, 144750, "2018-07-12", 1},
		{10, "Tim Berners-Lee", "tim.bernerslee@example.com", 3, 158300, "2013-10-01", 0},
	}
	for _, e := range emps {
		if _, err := db.Exec(`INSERT INTO employees(id,name,email,dept_id,salary,start_date,active) VALUES(?,?,?,?,?,?,?)`, e...); err != nil {
			return fmt.Errorf("employees: %w", err)
		}
	}

	sales := [][]any{
		{6001, "2024-01-08", "Mechanical Keyboard", 3, 269.97, "NA"},
		{6002, "2024-01-15", "27in Monitor", 2, 598.00, "EMEA"},
		{6003, "2024-02-02", "Noise-Cancel Headset", 1, 199.99, "APAC"},
		{6004, "2024-02-20", "Docking Station", 4, 758.00, "NA"},
		{6005, "2024-03-11", "External SSD 1TB", 5, 645.00, "LATAM"},
		{6006, "2024-03-29", "USB-C Hub", 6, 359.70, "EMEA"},
		{6007, "2024-04-14", "Wireless Mouse", 10, 345.00, "APAC"},
		{6008, "2024-05-02", "Laptop Stand", 7, 294.00, "NA"},
	}
	for _, s := range sales {
		if _, err := db.Exec(`INSERT INTO sales(order_id,order_date,product,quantity,net_total,region) VALUES(?,?,?,?,?,?)`, s...); err != nil {
			return fmt.Errorf("sales: %w", err)
		}
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "strata-gen:", err)
	os.Exit(1)
}
