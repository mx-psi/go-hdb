//go:build !unit

package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SAP/go-hdb/driver"
)

func testConnection(t *testing.T, db *sql.DB) {
	var dummy string
	err := db.QueryRow("select * from dummy").Scan(&dummy)
	switch {
	case err == sql.ErrNoRows:
		t.Fatal(err)
	case err != nil:
		t.Fatal(err)
	}
	if dummy != "X" {
		t.Fatalf("dummy is %s - expected %s", dummy, "X")
	}
}

func testPing(t *testing.T, db *sql.DB) {
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func testInsertByQuery(t *testing.T, db *sql.DB) {
	table := driver.RandomIdentifier("insertByQuery_")
	if _, err := db.Exec(fmt.Sprintf("create table %s (i integer)", table)); err != nil {
		t.Fatal(err)
	}

	// insert value via Query
	if err := db.QueryRow(fmt.Sprintf("insert into %s values (?)", table), 42).Scan(); err != sql.ErrNoRows {
		t.Fatal(err)
	}

	// check value
	var i int
	if err := db.QueryRow(fmt.Sprintf("select * from %s", table)).Scan(&i); err != nil {
		t.Fatal(err)
	}
	if i != 42 {
		t.Fatalf("value %d - expected %d", i, 42)
	}
}

func testHDBError(t *testing.T, db *sql.DB) {
	// select from not existing table with different table name length
	// to check if padding, etc works (see hint in protocol.error.Read(...))
	for i := 0; i < 9; i++ {
		_, err := db.Query(fmt.Sprintf("select * from %s", strings.Repeat("x", i+1))) //nolint:sqlclosecheck
		if err == nil {
			t.Fatal("hdb error expected")
		}
		var dbError driver.Error
		if !errors.As(err, &dbError) {
			t.Fatalf("hdb error expected got %v", err)
		}
		if dbError.Code() != 259 {
			t.Fatalf("hdb error code: %d - expected: %d", dbError.Code(), 259)
		}
	}
}

func testHDBWarning(t *testing.T, db *sql.DB) {
	// procedure gives warning:
	// 	SQL HdbWarning 1347 - Not recommended feature: DDL statement is used in Dynamic SQL (current dynamic_sql_ddl_error_level = 1)
	const procOut = `create procedure %[1]s ()
language SQLSCRIPT as
begin
	exec 'create table %[2]s(id int)';
	exec 'drop table %[2]s';
end
`
	procedure := driver.RandomIdentifier("proc_")
	tableName := driver.RandomIdentifier("table_")

	if _, err := db.Exec(fmt.Sprintf(procOut, procedure, tableName)); err != nil { // Create stored procedure.
		t.Fatal(err)
	}

	if _, err := db.Exec(fmt.Sprintf("call %s", procedure)); err != nil {
		t.Fatal(err)
	}
}

func testQueryAttributeAlias(t *testing.T, db *sql.DB) {
	table := driver.RandomIdentifier("queryAttributeAlias_")
	if _, err := db.Exec(fmt.Sprintf("create table %s (i integer, j integer)", table)); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(fmt.Sprintf("select i as x, j from %s", table))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	if columns[0] != "X" {
		t.Fatalf("value %s - expected %s", columns[0], "X")
	}

	if columns[1] != "J" {
		t.Fatalf("value %s - expected %s", columns[1], "J")
	}
}

func checkAffectedRows(t *testing.T, result sql.Result, rowsExpected int64) {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if rowsAffected != rowsExpected {
		t.Fatalf("rows affected %d - expected %d", rowsAffected, rowsExpected)
	}
}

func testRowsAffected(t *testing.T, db *sql.DB) {
	const maxRows = 10

	table := driver.RandomIdentifier("rowsAffected_")
	if _, err := db.Exec(fmt.Sprintf("create table %s (i integer)", table)); err != nil {
		t.Fatal(err)
	}

	stmt, err := db.Prepare(fmt.Sprintf("insert into %s values(?)", table))
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// insert
	for i := 0; i < maxRows; i++ {
		result, err := stmt.Exec(i)
		if err != nil {
			t.Fatal(err)
		}
		checkAffectedRows(t, result, 1)
	}

	// update
	result, err := db.Exec(fmt.Sprintf("update %s set i = %d where i <> %d", table, maxRows, maxRows))
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, maxRows)
}

func testUpsert(t *testing.T, db *sql.DB) {
	table := driver.RandomIdentifier("upsert_")
	if _, err := db.Exec(fmt.Sprintf("create table %s (key int primary key, val int)", table)); err != nil {
		t.Fatal(err)
	}

	result, err := db.Exec(fmt.Sprintf("upsert %s values (1, 1)", table))
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, 1)

	result, err = db.Exec(fmt.Sprintf("upsert %s values (:1, :1) where key = :2", table), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, 1)

	result, err = db.Exec(fmt.Sprintf("upsert %s values (?, ?) where key = ?", table), 1, 9, 1)
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, 1)

	result, err = db.Exec(fmt.Sprintf("upsert %s values (?, ?) with primary key", table), 1, 8)
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, 1)

	result, err = db.Exec(fmt.Sprintf("upsert %[1]s select key + ?, val from %[1]s", table), 2)
	if err != nil {
		t.Fatal(err)
	}
	checkAffectedRows(t, result, 2)
}

func testQueryArgs(t *testing.T, db *sql.DB) {
	table := driver.RandomIdentifier("table_")
	if _, err := db.Exec(fmt.Sprintf("create table %s (i integer, j integer)", table)); err != nil {
		t.Fatal(err)
	}

	var i = 0
	// positional args
	if err := db.QueryRow(fmt.Sprintf("select count(*) from %s where i = :1 and j = :1", table), 1).Scan(&i); err != nil {
		t.Fatal(err)
	}

	// mixed args
	if err := db.QueryRow(fmt.Sprintf("select count(*) from %s where i = ? and j = :3", table), 1, "arg not used", 2).Scan(&i); err != nil {
		t.Fatal(err)
	}
}

func testComments(t *testing.T, db *sql.DB) {
	tests := []struct {
		query     string
		supported bool
	}{
		{"select * from dummy\n-- my comment", true},
		{"-- my comment\nselect * from dummy", true},
		{"\n-- my comment\nselect * from dummy", true},
	}

	for _, test := range tests {
		rows, err := db.Query(test.query)
		if err != nil {
			if test.supported {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		}
		if rows != nil {
			rows.Close() //nolint:sqlclosecheck
		}
	}
}

func TestDriver(t *testing.T) {
	tests := []struct {
		name string
		fct  func(t *testing.T, db *sql.DB)
	}{
		{"connection", testConnection},
		{"ping", testPing},
		{"insertByQuery", testInsertByQuery},
		{"hdbError", testHDBError},
		{"hdbWarning", testHDBWarning},
		{"queryAttributeAlias", testQueryAttributeAlias},
		{"rowsAffected", testRowsAffected},
		{"upsert", testUpsert},
		{"queryArgs", testQueryArgs},
		{"queryComments", testComments},
	}

	db := driver.DefaultTestDB()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fct(t, db)
		})
	}
}
