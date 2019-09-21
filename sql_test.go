package main

import (
	"reflect"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestInsertSelectDB(t *testing.T) {
	defer SetupTestDB(t)()

	// DBに新規接続
	// openSQLとは異なり"DB"を取得する
	db := NewTestDB(t)
	inputs := [][]string{
		[]string{"1001", "2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"},
		[]string{"1001", "2019/05/15", "4841", "4854", "4781", "4854", "5077200", "4854.0"},
		[]string{"1001", "2019/05/14", "4780", "4873", "4775", "4870", "7363600", "4870.0"},
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
