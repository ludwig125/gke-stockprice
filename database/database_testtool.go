package database

import (
	"database/sql"
	"fmt"
	"log"
)

// SetupTestDB creates test Database and table
// cleanupTestDB関数を返すので、呼び出し元は"defer SetupTestDB(t)()"
// とするだけで、test用DatabaseとTableの作成と、テスト終了時の削除を担保できる
func SetupTestDB(port int) (func(), error) {
	if port == 0 {
		return nil, fmt.Errorf("port is not set. port '%d'", port)
	}
	log.Println("test database port", port)

	// DB名を指定せずに接続
	db, err := openSQL(fmt.Sprintf("root@tcp(localhost:%d)/", port))
	if err != nil {
		return nil, fmt.Errorf("failed to openSQL: %v", err)
	}
	// Database の作成
	if err := createTestDB(db); err != nil {
		return nil, fmt.Errorf("failed to createTestDB: %v", err)
	}
	db.Close()

	// Database 名を指定して接続し直す
	// ref: https://stackoverflow.com/questions/30235031/how-to-create-a-new-mysql-database-with-go-sql-driver
	db, err = openSQL(fmt.Sprintf("root@tcp(localhost:%d)/stockprice_dev", port))
	if err != nil {
		return nil, fmt.Errorf("failed to openSQL: %v", err)
	}

	// // stockprice_dev database が存在することを確認
	// if err := retry.Retry(3, 2*time.Second, func() error {
	// 	ok, err := existDatabases(db, "stockprice_dev")
	// 	if err != nil {
	// 		return fmt.Errorf("failed to existDatabases: %v", err)
	// 	}
	// 	if !ok {
	// 		return fmt.Errorf("failed to confirm database: stockprice_dev")
	// 	}
	// 	return nil
	// }); err != nil {
	// 	return nil, err
	// }

	// test用tableの作成
	if err := createTestTable(db); err != nil {
		return nil, fmt.Errorf("failed to createTestTable: %v", err)
	}

	// cleanupTestDB関数を返す
	return func() {
		cleanupTestDB(db)
	}, nil
}

// NewTestDB connect Test DB
func NewTestDB() (DB, error) {
	return NewDB("root@/stockprice_dev")
}

// test用Database作成
func createTestDB(db *sql.DB) error {
	// ref. https://medium.com/@udayakumarvdm/create-mysql-database-using-golang-b28c08e54660
	_, err := db.Exec("CREATE DATABASE IF NOT EXISTS stockprice_dev")
	if err != nil {
		return fmt.Errorf("failed to CREATE DATABASE: %v", err)
	}
	return nil
}

// test用Databaseの削除
func cleanupTestDB(db *sql.DB) error {
	// databaseをcloseする前にDROPしないと失敗する
	if err := dropTestDB(db); err != nil {
		return fmt.Errorf("failed to dropTestDB: %v", err)
	}
	db.Close()
	return nil
}

func dropTestDB(db *sql.DB) error {
	_, err := db.Exec("DROP DATABASE IF EXISTS stockprice_dev")
	if err != nil {
		return fmt.Errorf("failed to DROP DATABASE: %v", err)
	}
	return nil
}

// test用tableの作成
func createTestTable(db *sql.DB) error {
	tables := []string{
		`stockprice_dev.daily (
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
		`stockprice_dev.movingavg (
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
		`stockprice_dev.trend (
		code VARCHAR(10) NOT NULL,
		date VARCHAR(10) NOT NULL,
		trend VARCHAR(15),
		growthRate DOUBLE,
		crossMoving5 TINYINT(1),
		trendChange TINYINT(1),
		PRIMARY KEY( code, date )
	)`}
	for _, ddl := range tables {
		//log.Println("trying to create table", ddl)
		if _, err := db.Exec("CREATE TABLE IF NOT EXISTS " + ddl); err != nil {
			return fmt.Errorf("failed to create TestTable: %v", err)
		}
	}
	return nil
}

// func existDatabases(db *sql.DB, targetDB string) (bool, error) {
// 	databases, err := listDatabases(db)
// 	if err != nil {
// 		return false, fmt.Errorf("failed to listDatabases: %v", err)
// 	}
// 	for _, d := range databases {
// 		if d == targetDB {
// 			log.Println("databases:", databases, ", targetDB:", targetDB)
// 			return true, nil
// 		}
// 	}
// 	// 見つけられなかった場合はfalseを返す
// 	return false, nil
// }

// func listDatabases(db *sql.DB) ([]string, error) {
// 	rows, err := db.Query("SHOW DATABASES")
// 	if err != nil {
// 		return nil, fmt.Errorf("Could not query db: %v", err)
// 	}
// 	defer rows.Close()

// 	var databases []string
// 	for rows.Next() {
// 		var dbName string
// 		if err := rows.Scan(&dbName); err != nil {
// 			return nil, fmt.Errorf("Could not scan result: %v", err)
// 		}
// 		databases = append(databases, dbName)
// 	}
// 	return databases, nil
// }

// func listTables(db *sql.DB) ([]string, error) {
// 	rows, err := db.Query("SHOW TABLES")
// 	if err != nil {
// 		return nil, fmt.Errorf("Could not query db: %v", err)
// 	}
// 	defer rows.Close()

// 	var tables []string
// 	for rows.Next() {
// 		var dbName string
// 		if err := rows.Scan(&dbName); err != nil {
// 			return nil, fmt.Errorf("Could not scan result: %v", err)
// 		}
// 		tables = append(tables, dbName)
// 	}
// 	return tables, nil
// }
