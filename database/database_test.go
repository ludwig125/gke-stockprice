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
		{"1001", "2019/05/16", "100", "110", "120", "130", "140000", "150.0"},
		{"1001", "2019/05/15", "101", "111", "121", "131", "140001", "151.0"},
		{"1001", "2019/05/14", "102", "112", "122", "132", "140002", "152.0"},
	}
	// 以下のテストデータは上のinputsの100を1100にしたもの
	inputs2 := [][]string{
		{"1001", "2019/05/16", "1100", "110", "120", "130", "140000", "150.0"},
		{"1001", "2019/05/15", "1101", "111", "121", "131", "140001", "151.0"},
		{"1001", "2019/05/14", "1102", "112", "122", "132", "140002", "152.0"},
	}
	t.Run("insert", func(t *testing.T) {
		if err := db.InsertDB("daily", inputs); err != nil {
			t.Error(err)
		}
		ret, err := db.SelectDB("SELECT * FROM daily")
		if err != nil {
			t.Error(err)
		}
		// insert対象のデータとinsert後のテーブルからのselectの結果を比較
		if !compare2dSlices(ret, inputs) {
			t.Errorf("got %#v, want %#v", ret, inputs)
		}
	})

	t.Run("insert_duplicate_key_ignore", func(t *testing.T) {
		// InsertDBでinputs2を入れてみる
		if err := db.InsertDB("daily", inputs2); err != nil {
			t.Error(err)
		}
		ret, err := db.SelectDB("SELECT * FROM daily")
		if err != nil {
			t.Error(err)
		}
		// InsertDBの場合は INSERT IGNORE なので、すでにあるレコードはスキップされて最初のinputsと変化なし
		if !compare2dSlices(ret, inputs) {
			t.Errorf("got %#v, want %#v", ret, inputs)
		}
	})

	t.Run("insert_duplicate_key_update", func(t *testing.T) {
		// InsertOrUpdateDBでinputsを入れなおす
		if err := db.InsertOrUpdateDB("daily", inputs2); err != nil {
			t.Error(err)
		}
		ret, err := db.SelectDB("SELECT * FROM daily")
		if err != nil {
			t.Error(err)
		}
		// InsertOrUpdateDBの場合は INSERT ON DUPLICATE KEY UPDATE なので、すでにあるレコードもアップデートされる
		// なので、SElECT結果はinputs2になる
		if !compare2dSlices(ret, inputs2) {
			t.Errorf("got %#v, want %#v", ret, inputs2)
		}
	})
}

func compare2dSlices(ret, inputs [][]string) bool {
	// DeepEqualはsliceの順序までみて一致を判定するのでSliceをMapに変換する
	sliceToMap := func(s [][]string) map[string]bool {
		m := make(map[string]bool)
		for i := 0; i < len(s); i++ {
			// 簡単のため１レコードは全て結合して１つの文字列にしている
			m[strings.Join(s[i], "")] = true
		}
		return m
	}
	return reflect.DeepEqual(sliceToMap(ret), sliceToMap(inputs))
}
