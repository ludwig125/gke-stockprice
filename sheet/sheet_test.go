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

	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
	// spreadsheetのserviceを取得
	srv, err := sheet.GetSheetClient(ctx, "../"+sheetCredential)
	if err != nil {
		t.Fatalf("failed to get sheet service. err: %v", err)
	}
	log.Println("succeeded to get sheet service")

	ts := sheet.NewSpreadSheet(srv, mustGetenv("TEST_SHEET_ID"), "unittest")

	testdata := [][]string{
		[]string{"a", "b", "c"},
		[]string{"d", "e", "f"},
	}
	if err := ts.Update(testdata); err != nil {
		t.Error(err)
	}
	// testdata２つ分追加で書き込む
	if err := ts.Insert(append(testdata, testdata...)); err != nil {
		t.Error(err)
	}
	want := [][]string{
		[]string{"a", "b", "c"},
		[]string{"d", "e", "f"},
		[]string{"a", "b", "c"},
		[]string{"d", "e", "f"},
		[]string{"a", "b", "c"},
		[]string{"d", "e", "f"},
	}
	got, err := ts.Read()
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v want: %v", got, want)
	}

	// データを削除する
	if err := ts.Update([][]string{}); err != nil {
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

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set", k)
	}
	log.Printf("%s environment variable set", k)
	return v
}

// func TestRead(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
// 	// spreadsheetのserviceを取得
// 	srv, err := GetSheetClient(ctx, sheetCredential)
// 	if err != nil {
// 		t.Fatalf("failed to get sheet service. err: %v", err)
// 	}
// 	log.Println("succeeded to get sheet service")

// 	testSheetID := mustGetenv("TEST_SHEET_ID")
// 	t.Run("testSheet", func(t *testing.T) {
// 		testHolidaySheet(t, srv, testSheetID)
// 		testCodeSheet(t, srv, testSheetID)
// 	})
// }

// func testHolidaySheet(t *testing.T, srv *sheets.Service, sid string) {
// 	si := SpreadSheet{Service: srv,
// 		SpreadsheetID: sid,
// 		ReadRange:     "holiday",
// 	}
// 	resp, err := si.Read()
// 	if err != nil {
// 		t.Fatalf("failed to ReadSheet: %v", err)
// 	}
// 	t.Log(resp[0][0])
// 	for _, v := range resp {
// 		t.Log(v[0])
// 	}
// }

// func testCodeSheet(t *testing.T, srv *sheets.Service, sid string) {
// 	si := SpreadSheet{
// 		Service:       srv,
// 		SpreadsheetID: sid,
// 		ReadRange:     "tse-first",
// 	}
// 	resp, err := si.Read()
// 	if err != nil {
// 		t.Fatalf("failed to ReadSheet: %v", err)
// 	}
// 	log.Println(resp[0][0])
// 	t.Log("res", resp[0][0])
// 	os.Exit(0)
// 	for _, v := range resp {
// 		t.Log(v[0])
// 	}
// }

// func TestUpdate(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
// 	// spreadsheetのserviceを取得
// 	srv, err := GetSheetClient(ctx, sheetCredential)
// 	if err != nil {
// 		t.Fatalf("failed to get sheet service. err: %v", err)
// 	}
// 	log.Println("succeeded to get sheet service")
// 	sid := mustGetenv("TEST_SHEET_ID")

// 	si := SpreadSheet{
// 		Service:       srv,
// 		SpreadsheetID: sid,
// 		ReadRange:     "sample",
// 	}
// 	data := [][]string{
// 		[]string{"a", "b"},
// 		[]string{"c", "d"},
// 	}

// 	si.Update(data)
// }
