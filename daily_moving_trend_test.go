package main

import (
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
	sheets "google.golang.org/api/sheets/v4"
)

// spreadsheet mock data
var mockSheetData [][]string

type TrendSpreadSheetMock struct {
	Service       *sheets.Service
	SpreadsheetID string // sheetのID
	ReadRange     string // sheetのタブ名
}

func (s TrendSpreadSheetMock) Read() ([][]string, error) {
	return [][]string{}, nil
}

func (s TrendSpreadSheetMock) Insert([][]string) error {
	return nil
}

func (s TrendSpreadSheetMock) Update(ss [][]string) error {
	mockSheetData = ss
	return nil
}

func (s TrendSpreadSheetMock) Clear() error {
	return nil
}

func TestGrowthTrend(t *testing.T) {
	targetDateStr := "2020/12/20" // 実行する日。またはその時点のTrendを取得したい日
	targetDate, err := time.Parse("2006/01/02", targetDateStr)
	if err != nil {
		t.Fatal("failed to parse date", err)
	}

	tests := map[string]struct {
		dailyData             map[string][]DateClose
		longTermThresholdDays int
		wantCode              []string
	}{
		"success": {
			dailyData: map[string][]DateClose{
				"1011": makeDailyData("1011", targetDate, 1000, closeData{n: 100, r: 1}),                                                                           // ずっと増加
				"1012": makeDailyData("1012", targetDate, 1000, closeData{n: 100, r: -1}),                                                                          // ずっと減少
				"1013": makeDailyData("1013", targetDate, 1000, closeData{n: 50, r: 1}, closeData{n: 50, r: -1}),                                                   // 前半増加、後半減少
				"1014": makeDailyData("1014", targetDate, 1000, closeData{n: 50, r: -1}, closeData{n: 50, r: 1}),                                                   // 前半減少、後半増加
				"1015": makeDailyData("1015", targetDate, 1000, closeData{n: 80, r: 1}, closeData{n: 10, r: -1}, closeData{n: 9, r: 1}, closeData{n: 1, r: 100}),   // 増加してちょっと減少しまた増加、最後急激に増加
				"1016": makeDailyData("1016", targetDate, 1000, closeData{n: 80, r: -1}, closeData{n: 10, r: 1}, closeData{n: 9, r: -1}, closeData{n: 1, r: -100}), // 減少してちょっと増加しまた減少、最後急激に減少
				"1017": makeDailyData("1017", targetDate, 1000, closeData{n: 90, r: 1}, closeData{n: 10, r: -1}),                                                   // 増加してたが最後減少して方向感なし
				"1018": makeDailyData("1018", targetDate, 1000, closeData{n: 94, r: 1}, closeData{n: 6, r: -2}),                                                    // 増加してたが最後の数日だけ減少
				"1019": makeDailyData("1019", targetDate, 1000, closeData{n: 94, r: -1}, closeData{n: 6, r: 2}),                                                    // 減少してたが最後の数日だけ増加
				"1020": makeDailyData("1020", targetDate, 1000, closeData{n: 99}, closeData{n: 1, r: 2}),                                                           // ずっと横這い最後の1日だけ増加
				"1021": makeDailyData("1021", targetDate, 1000, closeData{n: 99}, closeData{n: 1, r: -2}),                                                          // ずっと横這い最後の1日だけ減少
				"1022": makeDailyData("1022", targetDate, 1000, closeData{n: 97, r: 1}, closeData{n: 3, r: -10}),                                                   // 最後だけ急落
				"1023": makeDailyData("1023", targetDate, 1000, closeData{n: 97, r: -1}, closeData{n: 3, r: 10}),                                                   // 最後だけ急騰
				// "1111": makeDailyData("1111", targetDate, 100000, closeData{n: 10000, r: 1}),                                                                       // ずっと増加
				// "1111": makeDailyData("1111", targetDate, 100000, closeData{n: 9998, r: 1}, closeData{n: 2, r: 21}), // ずっと増加
				// "1112": makeDailyData("1112", targetDate, 100000, closeData{n: 10000, r: -1}), // ずっと減少
			},
			longTermThresholdDays: 2,
			wantCode:              []string{"1015", "1011", "1020", "1014", "1023", "1019", "1017", "1018", "1022", "1013", "1021", "1012", "1016"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// ctx, cancel := context.WithCancel(context.Background())
			// defer cancel()

			var srv *sheets.Service
			trendSheet := TrendSpreadSheetMock{
				Service:       srv,
				SpreadsheetID: "aaa",
				ReadRange:     "bbb",
			}
			// 実際にSpreadSeetに書き込みたいときは以下を使う
			// // spreadsheetのserviceを取得
			// srv, err := getSheetService(ctx)
			// if err != nil {
			// 	t.Fatalf("failed to getSheetService: %v", err)
			// }
			// log.Println("got sheet service successfully")
			// trendSheet := sheet.NewSpreadSheet(srv, "<sheetID>", "trend")

			// DBの準備
			_, err := database.SetupTestDB(3306)
			// cleanup, err := database.SetupTestDB(3306) // cleanupを呼ばなければテスト後にDBを消さない
			if err != nil {
				t.Fatalf("failed to SetupTestDB: %v", err)
			}
			// defer cleanup()
			db, err := database.NewTestDB()
			if err != nil {
				t.Fatalf("failed to NewTestDB: %v", err)
			}

			var inputsDaily [][]string
			// 一気にinsertするため一つにまとめる
			for code, dateCloses := range tc.dailyData {
				inputsDaily = append(inputsDaily, convertDateClosesToStringSlice(code, dateCloses)...)

				// dms := calculateMovingAvg(dateCloses)
				// inputsMovingAvg = append(inputsMovingAvg, convertDateMovingAvgToStringSlice(code, dms)...)

				// tts := makePreviousTrendTables(code, targetDateStr, dms, dateCloses, tc.longTermThresholdDays)
				// inputsTrend = append(inputsTrend, convertTrendTablesToStringSlice(tts)...)
			}

			// insert daily test data to DB
			if err := db.InsertDB("daily", inputsDaily); err != nil {
				t.Fatalf("failed to insert daily: %v", err)
			}

			var codes []string
			for code := range tc.dailyData {
				codes = append(codes, code)
			}

			calc := CalculateDailyMovingAvgTrend{
				db:                    db,
				sheet:                 trendSheet,
				calcConcurrency:       3,
				targetDate:            targetDateStr,
				longTermThresholdDays: 2,
			}
			// calc, err := NewCalcMovingTrend(config)
			// if err != nil {
			// 	t.Errorf("failed to NewCalcMovingTrend: %v", err)
			// }
			if err := calc.Exec(codes); err != nil {
				t.Errorf("failed to Exec: %v", err)
			}

			var gotCodes []string
			for i, v := range mockSheetData {
				t.Log(v)
				if i == 0 {
					continue
				}
				gotCodes = append(gotCodes, v[0])
			}

			// 以下の形になるはず
			// [code trend trendTurn growthRate crossMoving5 continuationDays 20201220]
			// [1015 longTermAdvance upwardTurn 1.093 upwardCross 10]
			// [1011 longTermAdvance noTurn 1.001 noCross 10]
			// [1020 shortTermAdvance upwardTurn 1.002 upwardCross 1]
			// [1014 shortTermAdvance noTurn 1.001 noCross 10]
			// [1023 non upwardTurn 1.011 noCross 3]
			// [1019 non upwardTurn 1.002 noCross 6]
			// [1017 non noTurn 0.9991 noCross 10]
			// [1018 non downwardTurn 0.9982 noCross 6]
			// [1022 non downwardTurn 0.9907 noCross 3]
			// [1013 shortTermDecline noTurn 0.999 noCross 10]
			// [1021 shortTermDecline downwardTurn 0.998 downwardCross 1]
			// [1012 longTermDecline noTurn 0.9989 noCross 10]
			// [1016 longTermDecline downwardTurn 0.8914 downwardCross 10]
			if !reflect.DeepEqual(gotCodes, tc.wantCode) {
				t.Errorf("gotCodes: %v, wantCodes: %v", gotCodes, tc.wantCode)
			}
		})
	}
}

type closeData struct {
	n int
	r int
}

// numは常に正数
func (c closeData) num() int {
	return int(math.Abs(float64(c.n)))
}

// closeの増減率
func (c closeData) rate() int {
	return c.r //　設定されていないときは0を返す
}

// dailyテーブル用のテストデータを作成する関数
func makeDailyData(code string, targetDate time.Time, begin int, cs ...closeData) []DateClose {
	total := 0
	addAndSub := 0
	for _, c := range cs {
		// テストデータの件数を取得
		total += c.num()
		addAndSub += (c.num() * c.rate()) // n * rを足していってbeginより小さくならないか確認するため
	}

	// 最初の値に足し引きしてマイナスになる場合は全部正数にする
	if begin+addAndSub < 0 {
		log.Printf("Whoops, begin + input nums below zero!! begin: %d, input total: %d, sum=%d", begin, addAndSub, begin+addAndSub)
		var newCs []closeData
		for _, c := range cs {
			newCs = append(newCs, closeData{n: c.num(), r: c.rate()})
		}
		cs = newCs // 全部正数にする
	}

	// 終値のテストデータを作成
	var closes []string
	for _, c := range cs {
		end, cs := makeCloses(begin, c)
		closes = append(closes, cs...)
		begin = end
	}

	dateCloses := make([]DateClose, 0, total)
	for i := 0; i < total; i++ {
		date := targetDate.AddDate(0, 0, -i).Format("2006/01/02")
		// closesの末尾から順に直近の日付の終値として詰めていく
		close, err := strconv.ParseFloat(closes[len(closes)-1-i], 64)
		if err != nil {
			log.Panicf("failed to convert: %v", err)
		}
		dateCloses = append(dateCloses, DateClose{Date: date, Close: close})
	}
	return dateCloses
}

// dailyテーブルの終値テストデータを作成する関数
// nが正数のときはプラス方向に単調増加する数字を返す
// nが負数のときはマイナス方向に単調増加する数字を返す
func makeCloses(begin int, c closeData) (int, []string) {
	var s []string
	var end int
	for i := 1; i <= c.num(); i++ {
		end = begin + i*c.rate()
		s = append(s, fmt.Sprintf("%d", end))
	}
	return end, s
}

func convertDateClosesToStringSlice(code string, dateCloses []DateClose) [][]string {
	var ss [][]string
	for _, dateClose := range dateCloses {
		ss = append(ss, []string{code, dateClose.Date, "1", "1", "1", fmt.Sprintf("%0.f", dateClose.Close), "1", "1"}) // 小数点以下削除する
	}
	return ss
}
