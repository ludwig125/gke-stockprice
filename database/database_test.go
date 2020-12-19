// +build !integration

package database

import (
	"reflect"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestEnsureDB(t *testing.T) {
	//defer SetupTestDB(t, 3306)()
	cleanup, err := SetupTestDB(3306)
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer cleanup()

	db, err := NewTestDB()
	if err != nil {
		t.Fatalf("failed to NewTestDB: %v", err)
	}
	if err := ensureDB(db); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDBError(t *testing.T) {
	d, err := openSQL("root@/") // DB名を指定せずに接続
	if err != nil {
		t.Error(err)
	}
	db := &MySQL{d}
	wantErr := "database needs to be used. 'select database()': '[[]]'"
	if err := ensureDB(db); err != nil {
		if err.Error() != wantErr {
			t.Fatalf("got error: %v, want error: %v", err, wantErr)
		}
	}
}

func TestInsertSelectDB(t *testing.T) {
	//defer SetupTestDB(t, 3306)()
	cleanup, err := SetupTestDB(3306)
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer cleanup()

	// DBに新規接続
	// openSQLとは異なり"DB"を取得する
	//db := NewTestDB(t)
	db, err := NewTestDB()
	if err != nil {
		t.Fatalf("failed to NewTestDB: %v", err)
	}
	inputs := [][]string{
		{"1001", "2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"},
		{"1001", "2019/05/15", "4841", "4854", "4781", "4854", "5077200", "4854.0"},
		{"1001", "2019/05/14", "4780", "4873", "4775", "4870", "7363600", "4870.0"},
	}
	if err := db.InsertDB("daily", inputs); err != nil {
		t.Error(err)
	}

	ret, err := db.SelectDB("SELECT * FROM daily")
	if err != nil {
		t.Error(err)
	}

	// DeepEqualはsliceの順序までみて一致を判定するのでSliceをMapに変換する
	sliceToMap := func(s [][]string) map[string]bool {
		m := make(map[string]bool)
		for i := 0; i < len(s); i++ {
			// 簡単のため１レコードは全て結合して１つの文字列にしている
			m[strings.Join(s[i], "")] = true
		}
		return m
	}
	// insert対象のデータとinsert後のテーブルからのselectの結果を比較
	if !reflect.DeepEqual(sliceToMap(ret), sliceToMap(inputs)) {
		t.Errorf("got %#v, want %#v", ret, inputs)
	}
}
