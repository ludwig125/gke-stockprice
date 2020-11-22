// +build integration

package sheet_test

import (
	"context"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/ludwig125/gke-stockprice/sheet"
)

func TestSheet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sheetCredential := mustGetenv(t, "CREDENTIAL_FILEPATH")
	// spreadsheetのserviceを取得
	srv, err := sheet.GetSheetClient(ctx, "../"+sheetCredential)
	if err != nil {
		t.Fatalf("failed to get sheet service. err: %v", err)
	}
	log.Println("succeeded to get sheet service")

	ts := sheet.NewSpreadSheet(srv, mustGetenv(t, "INTEGRATION_TEST_SHEETID"), "unittest")

	testdata := [][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
	}
	if err := ts.Update(testdata); err != nil {
		t.Error(err)
	}
	// 同じtestdataを３つ分書き込む
	if err := ts.Insert(append(testdata, testdata...)); err != nil {
		t.Error(err)
	}
	want := [][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
		{"a", "b", "c"},
		{"d", "e", "f"},
		{"a", "b", "c"},
		{"d", "e", "f"},
	}
	got, err := ts.Read()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v want: %v", got, want)
	}

	// データを削除する
	if err := ts.Clear(); err != nil {
		t.Error(err)
	}
	got, err = ts.Read()
	if err != nil {
		t.Error(err)
	}
	// データがnilのことを確認
	var nilSlice [][]string
	if !reflect.DeepEqual(got, nilSlice) {
		t.Errorf("got: %#v want: %#v", got, nilSlice)
	}
}

func mustGetenv(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("%s environment variable not set", k)
	}
	return v
}
