package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"google.golang.org/api/sheets/v4"
	//_ "github.com/go-sql-driver/mysql"

	"github.com/ludwig125/gke-stockprice/database"
)

func TestFetchCompanyCode(t *testing.T) {
	var srv *sheets.Service
	codeSheet := CodeSpreadSheetMock{
		Service:       srv,
		SpreadsheetID: "aaa",
		ReadRange:     "bbb",
	}
	codes, err := fetchCompanyCode(codeSheet)
	if err != nil {
		t.Errorf("failed to fetchCompanyCode: %v", err)
	}
	want := []string{"100", "101", "102", "103", "104", "105", "106", "107"}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("got %v, want %v", codes, want)
	}
}

type CodeSpreadSheetMock struct {
	Service       *sheets.Service
	SpreadsheetID string // sheetのID
	ReadRange     string // sheetのタブ名
}

func (s CodeSpreadSheetMock) Read() ([][]string, error) {
	return [][]string{
		[]string{"100"},
		[]string{"101"},
		[]string{"102"},
		[]string{"103"},
		[]string{"104"},
		[]string{"105"},
		[]string{"106"},
		[]string{"107"},
	}, nil
}

func (s CodeSpreadSheetMock) Insert([][]string) error {
	return nil
}

func (s CodeSpreadSheetMock) Update([][]string) error {
	return nil
}

func TestFetchStockPrice(t *testing.T) {
	defer database.SetupTestDB(t, 3306)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testcodes := []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query()["scode"][0] // クエリからscodeの値を取得
		// TODO: scodeの値がない場合のテストどこかでする
		content, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s.html", code))
		if err != nil {
			t.Fatalf("failed to read testfile: %v", err)
		}
		fmt.Fprintf(w, string(content))
	}))
	defer ts.Close()

	db := database.NewTestDB(t)

	maxInsertDBNum := 3
	scrapeInterval := time.Duration(10) * time.Millisecond
	warns, err := fetchStockPrice(ctx, db, testcodes, ts.URL+"/", maxInsertDBNum, scrapeInterval)
	if warns != nil || err != nil {
		t.Errorf("warns: '%v', error: '%v'", warns, err)
	}

	// tableに格納されたcodeの数を確認
	retCodes, err := db.SelectDB("SELECT DISTINCT code FROM daily")
	if err != nil {
		t.Error(err)
	}
	if len(retCodes) != len(testcodes) {
		t.Errorf("got codes: %d, want: %d", len(retCodes), len(testcodes))
		t.Logf("codes: %v", retCodes)
	}

	// tableに格納された総レコード数を確認
	retRecords, err := db.SelectDB("SELECT COUNT(*) FROM daily")
	if err != nil {
		t.Error(err)
	}
	if retRecords[0][0] != fmt.Sprintf("%d", len(testcodes)*25) {
		t.Errorf("got records : %d, want: %d", len(retRecords), len(testcodes))
		t.Logf("records: %#v", retRecords[0][0])
	}
}

func TestCalculateMovingAvg(t *testing.T) {
	defer database.SetupTestDB(t, 3306)()

	db := database.NewTestDB(t)
	inputs := [][]string{
		[]string{"1011", "2019/10/18", "2886", "2913", "2857", "20", "15500", "2874.0"},
		[]string{"1011", "2019/10/17", "2907", "2907", "2878", "19", "15000", "2883.0"},
		[]string{"1011", "2019/10/16", "2902", "2932", "2892", "18", "23500", "2906.0"},
		[]string{"1011", "2019/10/15", "2845", "2905", "2845", "17", "28300", "2902.0"},
		[]string{"1011", "2019/10/11", "2879", "2879", "2820", "16", "21900", "2842.0"},
		[]string{"1011", "2019/10/10", "2876", "2876", "2844", "15", "11400", "2865.0"},
		[]string{"1011", "2019/10/09", "2850", "2865", "2827", "14", "27500", "2865.0"},
		[]string{"1011", "2019/10/08", "2813", "2896", "2813", "13", "48900", "2876.0"},
		[]string{"1012", "2019/10/18", "2886", "2913", "2857", "20", "15500", "2874.0"},
		[]string{"1012", "2019/10/17", "2907", "2907", "2878", "19", "15000", "2883.0"},
		[]string{"1012", "2019/10/16", "2902", "2932", "2892", "18", "23500", "2906.0"},
		[]string{"1012", "2019/10/15", "2845", "2905", "2845", "17", "28300", "2902.0"},
		[]string{"1012", "2019/10/11", "2879", "2879", "2820", "16", "21900", "2842.0"},
		[]string{"1012", "2019/10/10", "2876", "2876", "2844", "15", "11400", "2865.0"},
		[]string{"1012", "2019/10/09", "2850", "2865", "2827", "14", "27500", "2865.0"},
		[]string{"1012", "2019/10/08", "2813", "2896", "2813", "13", "48900", "2876.0"},
		[]string{"1013", "2019/10/18", "2886", "2913", "2857", "20", "15500", "2874.0"},
		[]string{"1013", "2019/10/17", "2907", "2907", "2878", "19", "15000", "2883.0"},
		[]string{"1013", "2019/10/16", "2902", "2932", "2892", "18", "23500", "2906.0"},
		[]string{"1013", "2019/10/15", "2845", "2905", "2845", "17", "28300", "2902.0"},
		[]string{"1013", "2019/10/11", "2879", "2879", "2820", "16", "21900", "2842.0"},
		[]string{"1013", "2019/10/10", "2876", "2876", "2844", "15", "11400", "2865.0"},
		[]string{"1013", "2019/10/09", "2850", "2865", "2827", "14", "27500", "2865.0"},
		[]string{"1013", "2019/10/08", "2813", "2896", "2813", "13", "48900", "2876.0"},
		[]string{"1014", "2019/10/18", "2886", "2913", "2857", "20", "15500", "2874.0"},
		[]string{"1014", "2019/10/17", "2907", "2907", "2878", "19", "15000", "2883.0"},
		[]string{"1014", "2019/10/16", "2902", "2932", "2892", "18", "23500", "2906.0"},
		[]string{"1014", "2019/10/15", "2845", "2905", "2845", "17", "28300", "2902.0"},
		[]string{"1014", "2019/10/11", "2879", "2879", "2820", "16", "21900", "2842.0"},
		[]string{"1014", "2019/10/10", "2876", "2876", "2844", "15", "11400", "2865.0"},
		[]string{"1014", "2019/10/09", "2850", "2865", "2827", "14", "27500", "2865.0"},
		[]string{"1014", "2019/10/08", "2813", "2896", "2813", "13", "48900", "2876.0"},
		[]string{"1015", "2019/10/18", "2886", "2913", "2857", "20", "15500", "2874.0"},
		[]string{"1015", "2019/10/17", "2907", "2907", "2878", "19", "15000", "2883.0"},
		[]string{"1015", "2019/10/16", "2902", "2932", "2892", "18", "23500", "2906.0"},
		[]string{"1015", "2019/10/15", "2845", "2905", "2845", "17", "28300", "2902.0"},
		[]string{"1015", "2019/10/11", "2879", "2879", "2820", "16", "21900", "2842.0"},
		[]string{"1015", "2019/10/10", "2876", "2876", "2844", "15", "11400", "2865.0"},
		[]string{"1015", "2019/10/09", "2850", "2865", "2827", "14", "27500", "2865.0"},
		[]string{"1015", "2019/10/08", "2813", "2896", "2813", "13", "48900", "2876.0"},
	}
	if err := db.InsertDB("daily", inputs); err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//codes := []string{"1011", "1012", "1013", "1014", "1015"}
	codes := []string{"1011", "1012"}
	if err := calculateMovingAvg(ctx, db, codes, 2); err != nil {
		t.Error(err)
	}
	want := [][]string{
		[]string{"1011", "2019/10/18", "19", "18", "17", "16.5", "16.5", "16.5", "16.5"},
		[]string{"1011", "2019/10/17", "18", "17", "16", "16", "16", "16", "16"},
		[]string{"1011", "2019/10/16", "17", "16", "15.5", "15.5", "15.5", "15.5", "15.5"},
		[]string{"1011", "2019/10/15", "16", "15", "15", "15", "15", "15", "15"},
		[]string{"1011", "2019/10/11", "15", "14.5", "14.5", "14.5", "14.5", "14.5", "14.5"},
		[]string{"1011", "2019/10/10", "14", "14", "14", "14", "14", "14", "14"},
		[]string{"1011", "2019/10/09", "13.5", "13.5", "13.5", "13.5", "13.5", "13.5", "13.5"},
		[]string{"1011", "2019/10/08", "13", "13", "13", "13", "13", "13", "13"},
		[]string{"1012", "2019/10/18", "19", "18", "17", "16.5", "16.5", "16.5", "16.5"},
		[]string{"1012", "2019/10/17", "18", "17", "16", "16", "16", "16", "16"},
		[]string{"1012", "2019/10/16", "17", "16", "15.5", "15.5", "15.5", "15.5", "15.5"},
		[]string{"1012", "2019/10/15", "16", "15", "15", "15", "15", "15", "15"},
		[]string{"1012", "2019/10/11", "15", "14.5", "14.5", "14.5", "14.5", "14.5", "14.5"},
		[]string{"1012", "2019/10/10", "14", "14", "14", "14", "14", "14", "14"},
		[]string{"1012", "2019/10/09", "13.5", "13.5", "13.5", "13.5", "13.5", "13.5", "13.5"},
		[]string{"1012", "2019/10/08", "13", "13", "13", "13", "13", "13", "13"},
	}
	got, err := db.SelectDB("SELECT * FROM movingavg ORDER BY code, date DESC")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}
