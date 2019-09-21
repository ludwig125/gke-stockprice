package main

import (
	"bytes"
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

// DB is interface of database
type DB interface {
	ShowDatabases() (string, error)
	InsertDB(table string, records [][]string) error
	SelectDB(q string) ([][]string, error)
	CloseDB() error
}

// MySQL is struct
type MySQL struct {
	db *sql.DB
}

// NewDB return new mysql database
/*
dataSourceName:
	cloud sql
		<user>:<pass>@cloudsql(<connection>)/<databaseName>
	local mysql
		root@/stockprice_dev
*/
func NewDB(dataSourceName string) (DB, error) {
	// dataSourceNameが与えられなければエラー
	if dataSourceName == "" {
		return nil, errors.New("dataSourceName no set")
	}
	sqldb, err := openSQL(dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to openSQL: %w", err)
	}
	// Open doesn't open a connection. Validate DSN data:
	if err = sqldb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to Ping: %w", err) // proper error handling instead of panic in your app
	}
	return &MySQL{sqldb}, nil
}

func openSQL(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}
	return db, nil
}

// ShowDatabases show all databases in mysql
func (m *MySQL) ShowDatabases() (string, error) {
	rows, err := m.db.Query("SHOW DATABASES")
	if err != nil {
		return "", fmt.Errorf("Could not query db: %v", err)
	}
	defer rows.Close()

	buf := bytes.NewBufferString("Databases:\n")
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return "", fmt.Errorf("Could not scan result: %v", err)
		}
		fmt.Fprintf(buf, "- %s\n", dbName)
	}
	return buf.String(), nil
}

// InsertDB insert data to database
func (m *MySQL) InsertDB(table string, records [][]string) error {
	// insert対象のtable名とレコードを引数に取ってDBに書き込む
	// 入力が空であればエラーを返す
	if len(records) == 0 {
		return fmt.Errorf("failed to insertDB. input is empty")
	}

	// insertのprebare作成
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "INSERT IGNORE INTO %s VALUES (", table)
	columns := len(records[0]) // 項目数だけ?をつなげる
	for i := 0; i < columns-1; i++ {
		buf.WriteString("?,")
	}
	buf.WriteString("?)")
	// Prepare statement for inserting data
	stmt, err := m.db.Prepare(buf.String()) // INSERT IGNORE INTO daily VALUES(?,?,?...,?)
	if err != nil {
		return fmt.Errorf("failed to Prepare: %v", err)
	}
	defer stmt.Close() // Close the statement when we leave

	colLen := len(records[0]) // １レコードあたりの項目数
	for i := 0; i < len(records); i++ {
		// Execは可変長引数のinterface型を受け取る
		// ref. https://github.com/go-sql-driver/mysql/issues/115
		cols := make([]interface{}, colLen)
		for c := 0; c < colLen; c++ {
			cols[c] = records[i][c]
		}
		_, err = stmt.Exec(cols...) // "1001", "2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"
		if err != nil {
			return fmt.Errorf("failed to Exec: %v", err)
		}
	}
	return nil
}

// TODO: 以下のようなことがあったのでRetry入れる
// failed to calcKahanshin. code: 5471, err: failed to getOrderedDateCloses. code: 5471, err: failed to selectTable failed to select. query: [SELECT date, close FROM daily WHERE code = 5471 AND date <= '2019/06/03' ORDER BY date DESC LIMIT 2;], err: invalid connection

// SelectDB select data from database
func (m *MySQL) SelectDB(q string) ([][]string, error) {
	rows, err := m.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to select. query: [%s], err: %v", q, err)
	}
	defer rows.Close()

	// 参考：https://github.com/go-sql-driver/mysql/wiki/Examples
	// テーブルから列名を取得する
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	// 列の長さ分だけのvalues
	// see https://golang.org/pkg/database/sql/#RawBytes
	// RawBytes is a byte slice that holds a reference to memory \
	// owned by the database itself.
	// After a Scan into a RawBytes, \
	// the slice is only valid until the next call to Next, Scan, or Close.
	values := make([]sql.RawBytes, len(columns))

	// rows.Scan は引数として'[]interface{}'が必要なので,
	// この引数scanArgsに列のサイズだけ確保した変数の参照をコピー
	// See http://code.google.com/p/go-wiki/wiki/InterfaceSlice for details
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	// select結果を詰める入れ物
	retVals := make([][]string, 0)

	// Fetch rows
	for rows.Next() {
		// get RawBytes from data
		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan: %v", err)
		}

		// １レコード単位のSlice
		rec := make([]string, 0)
		for _, col := range values {
			// Here we can check if the value is nil (NULL value)
			if col == nil {
				rec = append(rec, "")
			} else {
				rec = append(rec, string(col))
			}
		}
		retVals = append(retVals, rec)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row error: %v", err)
	}
	return retVals, nil
}

// CloseDB close database by db.Close()
func (m *MySQL) CloseDB() error {
	return m.db.Close()
}
