package main

import (
	"database/sql"
	"testing"
)

// SetupTestDB creates test Database and table
// cleanupTestDB関数を返すので、呼び出し元は"defer SetupTestDB(t)()"
// とするだけで、test用DatabaseとTableの作成と、テスト終了時の削除を担保できる
func SetupTestDB(t *testing.T) func() {
	db, err := openSQL("root@/") // DB名を指定せずに接続
	if err != nil {
		t.Error(err)
	}
	// Database の作成
	createTestDB(t, db)
	db.Close()

	// Database 名を指定して接続し直す
	// ref: https://stackoverflow.com/questions/30235031/how-to-create-a-new-mysql-database-with-go-sql-driver
	db, err = openSQL("root@/stockprice_dev")
	if err != nil {
		t.Error(err)
	}

	// test用tableの作成
	createTestTable(t, db)

	// cleanupTestDB関数を返す
	return func() {
		cleanupTestDB(t, db)
	}
}

// NewTestDB connect Test DB
func NewTestDB(t *testing.T) DB {
	db, err := NewDB("root@/stockprice_dev")
	if err != nil {
		t.Errorf("failed to NewDB: %v", err)
	}
	return db
}

// test用Database作成
func createTestDB(t *testing.T, db *sql.DB) {
	// ref. https://medium.com/@udayakumarvdm/create-mysql-database-using-golang-b28c08e54660
	_, err := db.Exec("CREATE DATABASE IF NOT EXISTS stockprice_dev")
	if err != nil {
		t.Fatalf("failed to createTestDB: %v", err)
	}
}

// test用Databaseの削除
func cleanupTestDB(t *testing.T, db *sql.DB) {
	dropTestDB(t, db) // databaseをcloseする前にDROPしないと失敗する
	db.Close()
}

func dropTestDB(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP DATABASE IF EXISTS stockprice_dev")
	if err != nil {
		t.Fatalf("failed to dropTestDB: %v", err)
	}
}

// test用tableの作成
func createTestTable(t *testing.T, db *sql.DB) {
	tables := []string{
		`daily (
		code VARCHAR(10) NOT NULL,
		date VARCHAR(10) NOT NULL,
		open VARCHAR(15),
		high VARCHAR(15),
		low VARCHAR(15),
		close VARCHAR(15),
		turnover VARCHAR(15),
		modified VARCHAR(15),
		PRIMARY KEY( code, date )
	)`,
		`movingavg (
        code VARCHAR(10) NOT NULL,
        date VARCHAR(10) NOT NULL,
        moving3 DOUBLE,
        moving5 DOUBLE,
        moving7 DOUBLE,
        moving10 DOUBLE,
        moving20 DOUBLE,
        moving60 DOUBLE,
        moving100 DOUBLE,
        PRIMARY KEY( code, date )
	)`,
	}
	for _, table := range tables {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + table)
		if err != nil {
			t.Fatalf("failed to createTestTable: %v", err)
		}
	}
}
