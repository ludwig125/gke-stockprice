// +build !integration

package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
)

func TestRestructure(t *testing.T) {
	targetDateStr := "2020/12/20" // 実行する日
	targetDate, err := time.Parse("2006/01/02", targetDateStr)
	if err != nil {
		t.Fatal("failed to parse date", err)
	}
	dailyData := map[string][]DateClose{
		"1011": makeDailyData("1011", targetDate, 1000, closeData{n: 6, r: 1}),
		"1012": makeDailyData("1012", targetDate, 1000, closeData{n: 6, r: -1}),
		"1013": makeDailyData("1013", targetDate, 1000, closeData{n: 3, r: 1}, closeData{n: 3, r: -1}),
		"1014": makeDailyData("1014", targetDate, 1000, closeData{n: 3, r: -1}, closeData{n: 3, r: 1}),
		"1015": makeDailyData("1015", targetDate, 1000, closeData{n: 2, r: 1}, closeData{n: 2, r: 1}, closeData{n: 2, r: -1}),
		"1016": makeDailyData("1016", targetDate, 1000, closeData{n: 2, r: 1}, closeData{n: 2, r: -1}, closeData{n: 2, r: 1}),
		"1017": makeDailyData("1017", targetDate, 1000, closeData{n: 2}, closeData{n: 2, r: 1}, closeData{n: 2, r: -1}),
		"1018": makeDailyData("1018", targetDate, 1000, closeData{n: 2}, closeData{n: 2, r: -1}, closeData{n: 2, r: 1}),
	}
	wantMovingAvg := [][]string{
		{"1011", "2020/12/15", "1001", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1011", "2020/12/16", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5"},
		{"1011", "2020/12/17", "1002", "1002", "1002", "1002", "1002", "1002", "1002"},
		{"1011", "2020/12/18", "1003", "1002.5", "1002.5", "1002.5", "1002.5", "1002.5", "1002.5"},
		{"1011", "2020/12/19", "1004", "1003", "1003", "1003", "1003", "1003", "1003"},
		{"1011", "2020/12/20", "1005", "1004", "1003.5", "1003.5", "1003.5", "1003.5", "1003.5"},
		{"1012", "2020/12/15", "999", "999", "999", "999", "999", "999", "999"},
		{"1012", "2020/12/16", "998.5", "998.5", "998.5", "998.5", "998.5", "998.5", "998.5"},
		{"1012", "2020/12/17", "998", "998", "998", "998", "998", "998", "998"},
		{"1012", "2020/12/18", "997", "997.5", "997.5", "997.5", "997.5", "997.5", "997.5"},
		{"1012", "2020/12/19", "996", "997", "997", "997", "997", "997", "997"},
		{"1012", "2020/12/20", "995", "996", "996.5", "996.5", "996.5", "996.5", "996.5"},
		{"1013", "2020/12/15", "1001", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1013", "2020/12/16", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5"},
		{"1013", "2020/12/17", "1002", "1002", "1002", "1002", "1002", "1002", "1002"},
		{"1013", "2020/12/18", "1002.3333333333334", "1002", "1002", "1002", "1002", "1002", "1002"},
		{"1013", "2020/12/19", "1002", "1001.8", "1001.8", "1001.8", "1001.8", "1001.8", "1001.8"},
		{"1013", "2020/12/20", "1001", "1001.6", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5"},
		{"1014", "2020/12/15", "999", "999", "999", "999", "999", "999", "999"},
		{"1014", "2020/12/16", "998.5", "998.5", "998.5", "998.5", "998.5", "998.5", "998.5"},
		{"1014", "2020/12/17", "998", "998", "998", "998", "998", "998", "998"},
		{"1014", "2020/12/18", "997.6666666666666", "998", "998", "998", "998", "998", "998"},
		{"1014", "2020/12/19", "998", "998.2", "998.2", "998.2", "998.2", "998.2", "998.2"},
		{"1014", "2020/12/20", "999", "998.4", "998.5", "998.5", "998.5", "998.5", "998.5"},
		{"1015", "2020/12/15", "1001", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1015", "2020/12/16", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5"},
		{"1015", "2020/12/17", "1002", "1002", "1002", "1002", "1002", "1002", "1002"},
		{"1015", "2020/12/18", "1003", "1002.5", "1002.5", "1002.5", "1002.5", "1002.5", "1002.5"},
		{"1015", "2020/12/19", "1003.3333333333334", "1002.6", "1002.6", "1002.6", "1002.6", "1002.6", "1002.6"},
		{"1015", "2020/12/20", "1003", "1002.8", "1002.5", "1002.5", "1002.5", "1002.5", "1002.5"},
		{"1016", "2020/12/15", "1001", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1016", "2020/12/16", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5", "1001.5"},
		{"1016", "2020/12/17", "1001.3333333333334", "1001.3333333333334", "1001.3333333333334", "1001.3333333333334", "1001.3333333333334", "1001.3333333333334", "1001.3333333333334"},
		{"1016", "2020/12/18", "1001", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1016", "2020/12/19", "1000.6666666666666", "1001", "1001", "1001", "1001", "1001", "1001"},
		{"1016", "2020/12/20", "1001", "1001.2", "1001.1666666666666", "1001.1666666666666", "1001.1666666666666", "1001.1666666666666", "1001.1666666666666"},
		{"1017", "2020/12/15", "1000", "1000", "1000", "1000", "1000", "1000", "1000"},
		{"1017", "2020/12/16", "1000", "1000", "1000", "1000", "1000", "1000", "1000"},
		{"1017", "2020/12/17", "1000.3333333333334", "1000.3333333333334", "1000.3333333333334", "1000.3333333333334", "1000.3333333333334", "1000.3333333333334", "1000.3333333333334"},
		{"1017", "2020/12/18", "1001", "1000.75", "1000.75", "1000.75", "1000.75", "1000.75", "1000.75"},
		{"1017", "2020/12/19", "1001.3333333333334", "1000.8", "1000.8", "1000.8", "1000.8", "1000.8", "1000.8"},
		{"1017", "2020/12/20", "1001", "1000.8", "1000.6666666666666", "1000.6666666666666", "1000.6666666666666", "1000.6666666666666", "1000.6666666666666"},
		{"1018", "2020/12/15", "1000", "1000", "1000", "1000", "1000", "1000", "1000"},
		{"1018", "2020/12/16", "1000", "1000", "1000", "1000", "1000", "1000", "1000"},
		{"1018", "2020/12/17", "999.6666666666666", "999.6666666666666", "999.6666666666666", "999.6666666666666", "999.6666666666666", "999.6666666666666", "999.6666666666666"},
		{"1018", "2020/12/18", "999", "999.25", "999.25", "999.25", "999.25", "999.25", "999.25"},
		{"1018", "2020/12/19", "998.6666666666666", "999.2", "999.2", "999.2", "999.2", "999.2", "999.2"},
		{"1018", "2020/12/20", "999", "999.2", "999.3333333333334", "999.3333333333334", "999.3333333333334", "999.3333333333334", "999.3333333333334"},
	}
	wantTrend := [][]string{
		{"1011", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1011", "2020/12/16", "3", "2", "1.001", "3", "1"},
		{"1011", "2020/12/17", "3", "2", "1.001", "2", "2"},
		{"1011", "2020/12/18", "3", "2", "1.001", "2", "3"},
		{"1011", "2020/12/19", "3", "2", "1.001", "2", "4"},
		{"1011", "2020/12/20", "3", "2", "1.001", "2", "5"},
		{"1012", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1012", "2020/12/16", "3", "2", "0.999", "1", "1"},
		{"1012", "2020/12/17", "3", "2", "0.999", "2", "2"},
		{"1012", "2020/12/18", "3", "2", "0.999", "2", "3"},
		{"1012", "2020/12/19", "3", "2", "0.999", "2", "4"},
		{"1012", "2020/12/20", "3", "2", "0.999", "2", "5"},
		{"1013", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1013", "2020/12/16", "3", "2", "1.001", "3", "1"},
		{"1013", "2020/12/17", "3", "2", "1.001", "2", "2"},
		{"1013", "2020/12/18", "3", "2", "0.999", "2", "1"},
		{"1013", "2020/12/19", "3", "2", "0.999", "1", "2"},
		{"1013", "2020/12/20", "3", "2", "0.999", "2", "3"},
		{"1014", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1014", "2020/12/16", "3", "2", "0.999", "1", "1"},
		{"1014", "2020/12/17", "3", "2", "0.999", "2", "2"},
		{"1014", "2020/12/18", "3", "2", "1.001", "2", "1"},
		{"1014", "2020/12/19", "3", "2", "1.001", "3", "2"},
		{"1014", "2020/12/20", "3", "2", "1.001", "2", "3"},
		{"1015", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1015", "2020/12/16", "3", "2", "1.001", "3", "1"},
		{"1015", "2020/12/17", "3", "2", "1.001", "2", "2"},
		{"1015", "2020/12/18", "3", "2", "1.001", "2", "3"},
		{"1015", "2020/12/19", "3", "2", "0.999", "2", "1"},
		{"1015", "2020/12/20", "3", "2", "0.999", "1", "2"},
		{"1016", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1016", "2020/12/16", "3", "2", "1.001", "3", "1"},
		{"1016", "2020/12/17", "3", "2", "0.999", "1", "1"},
		{"1016", "2020/12/18", "3", "2", "0.999", "2", "2"},
		{"1016", "2020/12/19", "3", "2", "1.001", "2", "1"},
		{"1016", "2020/12/20", "3", "2", "1.001", "3", "2"},
		{"1017", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1017", "2020/12/16", "3", "2", "1", "2", "0"},
		{"1017", "2020/12/17", "3", "2", "1.001", "3", "1"},
		{"1017", "2020/12/18", "3", "2", "1.001", "2", "2"},
		{"1017", "2020/12/19", "3", "2", "0.999", "2", "1"},
		{"1017", "2020/12/20", "3", "2", "0.999", "1", "2"},
		{"1018", "2020/12/15", "3", "0", "0", "2", "0"},
		{"1018", "2020/12/16", "3", "2", "1", "2", "0"},
		{"1018", "2020/12/17", "3", "2", "0.999", "1", "1"},
		{"1018", "2020/12/18", "3", "2", "0.999", "2", "2"},
		{"1018", "2020/12/19", "3", "2", "1.001", "2", "1"},
		{"1018", "2020/12/20", "3", "2", "1.001", "3", "2"},
	}

	var inputsDaily [][]string
	for code, dateCloses := range dailyData {
		inputsDaily = append(inputsDaily, convertDateClosesToStringSlice(code, dateCloses)...)
	}

	// DBの準備
	_, err = database.SetupTestDB(3306)
	// cleanup, err := database.SetupTestDB(3306) // cleanupを呼ばなければテスト後にDBを消さない
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	// defer cleanup()
	db, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("failed to NewTestDB: %v", err)
	}
	if err := db.InsertDB("daily", inputsDaily); err != nil {
		t.Fatalf("failed to insert daily: %v", err)
	}

	tests := map[string]struct {
		config     RestructureTablesFromDailyConfig
		wantErr    bool
		wantmoving [][]string
		wanttrend  [][]string
	}{
		"MaxConcurrency_1": {
			config: RestructureTablesFromDailyConfig{
				DB:                   db,
				DailyTable:           "daily",
				MovingAvgTable:       "movingavg",
				TrendTable:           "trend",
				Codes:                []string{"1011", "1012", "1013", "1014", "1015", "1016", "1017", "1018"},
				RestructureMovingavg: true,
				RestructureTrend:     true,
				FromDate:             "2020/12/15",
				ToDate:               "2020/12/20",
				MaxConcurrency:       1,
			},
			wantErr:    false,
			wantmoving: wantMovingAvg,
			wanttrend:  wantTrend,
		},
		"MaxConcurrency_3": {
			config: RestructureTablesFromDailyConfig{
				DB:                   db,
				DailyTable:           "daily",
				MovingAvgTable:       "movingavg",
				TrendTable:           "trend",
				Codes:                []string{"1011", "1012", "1013", "1014", "1015", "1016", "1017", "1018"},
				RestructureMovingavg: true,
				RestructureTrend:     true,
				FromDate:             "2020/12/15",
				ToDate:               "2020/12/20",
				MaxConcurrency:       3,
			},
			wantErr:    false,
			wantmoving: wantMovingAvg,
			wanttrend:  wantTrend,
		},
		"MaxConcurrency_3_targetCode_1011_1013_1015": {
			config: RestructureTablesFromDailyConfig{
				DB:                   db,
				DailyTable:           "daily",
				MovingAvgTable:       "movingavg",
				TrendTable:           "trend",
				Codes:                []string{"1011", "1013", "1015"},
				RestructureMovingavg: true,
				RestructureTrend:     true,
				FromDate:             "2020/12/15",
				ToDate:               "2020/12/20",
				MaxConcurrency:       3,
			},
			wantErr:    false,
			wantmoving: filterTargetCodeData(wantMovingAvg, []string{"1011", "1013", "1015"}),
			wanttrend:  filterTargetCodeData(wantTrend, []string{"1011", "1013", "1015"}),
		},
		// "filter_date": {
		// 	config: RestructureTablesFromDailyConfig{
		// 		DB:                   db,
		// 		DailyTable:           "daily",
		// 		MovingAvgTable:       "movingavg",
		// 		TrendTable:           "trend",
		// 		Codes:                []string{"1011", "1012", "1013", "1014", "1015", "1016", "1017", "1018"},
		// 		RestructureMovingavg: true,
		// 		RestructureTrend:     true,
		// 		FromDate:             "2020/12/16",
		// 		ToDate:               "2020/12/19",
		// 		MaxConcurrency:       3,
		// 	},
		// 	wantErr:    false,
		// 	wantmoving: filterDate(wantMovingAvg, "2020/12/16", "2020/12/19"),
		// 	wanttrend:  filterDate(wantTrend, "2020/12/16", "2020/12/19"),
		// },
		"invalidTable": {
			config: RestructureTablesFromDailyConfig{
				DB:                   db,
				DailyTable:           "daily",
				MovingAvgTable:       "movingavg_test",
				TrendTable:           "trend_test",
				Codes:                []string{"1011", "1013", "1015"},
				RestructureMovingavg: true,
				RestructureTrend:     true,
				FromDate:             "2020/12/15",
				ToDate:               "2020/12/20",
				MaxConcurrency:       3,
			},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer db.DeleteFromDB(tc.config.MovingAvgTable, tc.config.Codes)
			defer db.DeleteFromDB(tc.config.TrendTable, tc.config.Codes)

			r, err := NewRestructureTablesFromDaily(tc.config)
			if err != nil {
				t.Fatalf("failed to NewRestructureTablesFromDaily: %v", err)
			}
			err = r.Restructure()
			if (err != nil) != tc.wantErr {
				t.Errorf("error: %v, wantErr: %t", err, tc.wantErr)
				return
			}
			if err != nil {
				t.Log(err)
				return // エラーがある場合はこのあとの処理はしない
			}

			moving, err := db.SelectDB("SELECT * FROM " + tc.config.MovingAvgTable)
			if err != nil {
				t.Error(err)
			}
			if !compare2dSlices(moving, tc.wantmoving) {
				for _, v := range moving {
					fmt.Println("moving", v)
				}
				t.Errorf("got %#v, want %#v", moving, tc.wantmoving)
			}

			trend, err := db.SelectDB("SELECT * FROM " + tc.config.TrendTable)
			if err != nil {
				t.Error(err)
			}
			if !compare2dSlices(trend, tc.wanttrend) {
				t.Errorf("got %#v, want %#v", trend, tc.wanttrend)
				// diff := cmp.Diff(trend, tc.wanttrend)
				// t.Errorf("got %#v\nwant %#v\n%v", trend, tc.wanttrend, diff)
			}
		})
	}
}

func filterTargetCodeData(data [][]string, targetCodes []string) [][]string {
	var ss [][]string
	for _, v := range data {
		if isInTargetCode(v[0], targetCodes) {
			ss = append(ss, v)
		}
	}
	return ss
}

func isInTargetCode(code string, targetCodes []string) bool {
	for _, c := range targetCodes {
		if c == code {
			return true
		}
	}
	return false
}

// TODO: 単純にフィルターするとだめ
// 日付が変わった分、movingなどの計算結果も変わる
// func filterDate(data [][]string, fromDate, toDate string) [][]string {
// 	var ss [][]string
// 	for _, v := range data {
// 		date := v[1]
// 		if date >= fromDate && date <= toDate {
// 			// fmt.Println("date", date, fromDate, toDate)
// 			ss = append(ss, v)
// 		}
// 	}
// 	return ss
// }

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
