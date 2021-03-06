package database

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/pkg/errors"
	// ref: https://ja.stackoverflow.com/questions/12324/go%E3%81%AE%E3%83%87%E3%83%BC%E3%82%BF%E3%83%99%E3%83%BC%E3%82%B9%E6%8E%A5%E7%B6%9A%E3%81%AF%E3%81%A9%E3%81%93%E3%81%AB%E6%9B%B8%E3%81%91%E3%81%B0%E3%81%84%E3%81%84%E3%81%AE%E3%81%A7%E3%81%97%E3%82%87%E3%81%86%E3%81%8B
	// https://www.calhoun.io/why-we-import-sql-drivers-with-the-blank-identifier/
)

// DB is interface of database
type DB interface {
	ShowDatabases() (string, error)
	InsertDB(table string, records [][]string) error
	InsertOrUpdateDB(table string, records [][]string) error
	SelectDB(q string) ([][]string, error)
	DeleteFromDB(table string, codes []string) error
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
		<user>:<pass>@tcp(<dbinstance>)/<databaseName>
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
		return nil, fmt.Errorf("failed to Ping: %w", err)
	}

	// MySQLのコネクションが時間がたってInvalidにならないための工夫
	// https://www.alexedwards.net/blog/configuring-sqldb
	// https://making.pusher.com/production-ready-connection-pooling-in-go/
	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(25)
	sqldb.SetConnMaxLifetime(5 * time.Minute)

	db := &MySQL{sqldb}
	// DBに接続されているか確認
	if err := ensureDB(db); err != nil {
		return nil, fmt.Errorf("failed to ensureDB: %v", err)
	}
	return db, nil
}

func openSQL(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}

	return db, nil
}

func ensureDB(db DB) error {
	// ensure using database
	res, err := db.SelectDB("SELECT database()")
	if err != nil {
		return fmt.Errorf("failed to SELECT database(): %v", err)
	}
	// database が指定されていなかったらNULLが返る
	if res[0][0] == "" {
		return fmt.Errorf("database needs to be used. 'select database()': '%v'", res)
	}
	log.Printf("use database %s", res[0][0])
	return nil
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
	// データがなかったらINSERTして欲しいけど既に入っている場合には何もして欲しくない
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
		// TODO: 以下で捨てているresのRowsAffectedを確認する？
		_, err = stmt.Exec(cols...) // "1001", "2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"
		if err != nil {
			return fmt.Errorf("failed to Exec: %v", err)
		}
	}
	return nil
}

// InsertOrUpdateDB insert data to database
func (m *MySQL) InsertOrUpdateDB(table string, records [][]string) error {
	// insert対象のtable名とレコードを引数に取ってDBに書き込む
	// 入力が空であればエラーを返す
	if len(records) == 0 {
		return fmt.Errorf("failed to InsertOrUpdateDB. input is empty")
	}

	res, err := m.SelectDB(fmt.Sprintf("SELECT column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME='%s'", table))
	if err != nil {
		return fmt.Errorf("failed to fetch column name: %v", err)
	}
	if len(res) == 0 {
		return fmt.Errorf("got no column name")
	}
	var columnName []string
	for _, r := range res {
		columnName = append(columnName, r[0])
	}

	// insertのprebare作成
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "INSERT INTO %s VALUES (", table)
	columns := len(records[0]) // 項目数だけ?をつなげる
	for i := 0; i < columns-1; i++ {
		buf.WriteString("?,")
	}
	buf.WriteString("?) ON DUPLICATE KEY UPDATE ")

	for i := 0; i < len(columnName)-1; i++ {
		key := columnName[i]
		buf.WriteString(fmt.Sprintf("%s = VALUES(%s), ", key, key)) // カンマ繋がりで複数指定
	}
	key := columnName[len(columnName)-1]
	buf.WriteString(fmt.Sprintf("%s = VALUES(%s)", key, key)) // 最後のkey

	// fmt.Println("InsertOrUpdateDB prepare:", buf.String()) // prepare

	// Prepare statement for inserting data
	stmt, err := m.db.Prepare(buf.String()) // INSERT INTO daily VALUES(?,?,?...,?) ON DUPLICATE KEY UPDATE
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
		// TODO: 以下で捨てているresのRowsAffectedを確認する？
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

// DeleteFromDB delete data from database
// Truncateメソッドを作らなかったのは事故を防ぐため
func (m *MySQL) DeleteFromDB(table string, codes []string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE code=?", table)
	stmtDelete, err := m.db.Prepare(q)
	if err != nil {
		return fmt.Errorf("failed to Prepare: %w", err)
	}
	defer stmtDelete.Close()

	for _, code := range codes {
		result, err := stmtDelete.Exec(code)
		if err != nil {
			return fmt.Errorf("failed to delete Exec: %w, code: %s", err, code)
		}

		rowsAffect, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get RowsAffected: %w, code: %s", err, code)
		}

		// 何も消されたものがなければエラーにする
		if rowsAffect == 0 {
			return fmt.Errorf("failed to delete code: %s from table: %s. no affected", code, table)
		}
	}
	return nil
}

// CloseDB close database by db.Close()
func (m *MySQL) CloseDB() error {
	return m.db.Close()
}
