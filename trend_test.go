package main

import (
	"context"
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
	return nil
}

func (s TrendSpreadSheetMock) Clear() error {
	return nil
}

type CloseTestData struct {
	Num  int
	Rate int
}

// numは常に正数
func (c CloseTestData) num() int {
	return int(math.Abs(float64(c.Num)))
}

// closeの増減率
func (c CloseTestData) rate() int {
	if c.Rate == 0 { //　設定されていないときは+１を返す
		return 1
	}
	return c.Rate
}

// dailyテーブル用のテストデータを作成する関数
func makeDailyData(code string, previousDate time.Time, begin int, cs ...CloseTestData) [][]string {
	total := 0
	addAndSub := 0
	for _, c := range cs {
		// テストデータの件数を取得
		total += c.num()
		addAndSub += (c.num() * c.rate()) // Num * Rateを足していってbeginより小さくならないか確認するため
	}

	// 最初の値に足し引きしてマイナスになる場合は全部正数にする
	if begin+addAndSub < 0 {
		log.Printf("Whoops, begin + input nums below zero!! begin: %d, input total: %d, sum=%d", begin, addAndSub, begin+addAndSub)
		var newCs []CloseTestData
		for _, c := range cs {
			newCs = append(newCs, CloseTestData{Num: c.num(), Rate: c.rate()})
		}
		cs = newCs // 全部正数にする
	}

	// 終値のテストデータを作成
	var closes []string
	for _, c := range cs {
		end, cs := makeCloses(begin, c)
		//fmt.Println(cs)
		closes = append(closes, cs...)
		begin = end
	}

	var dailyData [][]string
	for i := 0; i < total; i++ {
		date := previousDate.AddDate(0, 0, -i).Format("2006/01/02")
		// closesの末尾から順に直近の日付の終値として詰めていく
		dailyData = append(dailyData, []string{code, date, "1", "1", "1", closes[len(closes)-1-i], "1", "1"})
	}
	return dailyData
}

// dailyテーブルの終値テストデータを作成する関数
// nが正数のときはプラス方向に単調増加する数字を返す
// nが負数のときはマイナス方向に単調増加する数字を返す
func makeCloses(begin int, c CloseTestData) (int, []string) {
	var s []string
	var end int
	for i := 1; i <= c.num(); i++ {
		end = begin + i*c.rate()
		s = append(s, fmt.Sprintf("%d", end))
	}
	return end, s
}

func makeMovingAvgDataFromDailyTestData(daily [][]string) [][]string {
	var dcs DateCloses
	for _, d := range daily {
		f, _ := strconv.ParseFloat(d[5], 64)
		dcs = append(dcs, DateClose{Date: d[1], Close: f})
	}

	moving := make(map[int]map[string]float64)
	for _, days := range targetMovingAvgs {
		moving[days] = dcs.calcMovingAvg(days)
	}

	str := func(f float64) string {
		return fmt.Sprintf("%g", f)
	}

	var ss [][]string
	for _, dc := range dcs {
		d := dc.Date
		ss = append(ss, []string{daily[0][0], d, str(moving[3][d]), str(moving[5][d]), str(moving[7][d]), str(moving[10][d]), str(moving[20][d]), str(moving[60][d]), str(moving[100][d])})
	}

	return ss
}

func TestGrowthTrend(t *testing.T) {
	cleanup, err := database.SetupTestDB(3306)
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer cleanup()
	db, err := database.NewTestDB()
	if err != nil {
		t.Fatalf("failed to NewTestDB: %v", err)
	}

	previousDate, err := time.Parse("2006-01-02", "2020-03-05") // 2020/3/5を前日として設定
	if err != nil {
		t.Fatal(err)
	}
	dailyData := map[string][][]string{
		"1011": makeDailyData("1011", previousDate, 1000, CloseTestData{Num: 100}),                                  // ずっと増加
		"1012": makeDailyData("1012", previousDate, 1000, CloseTestData{Num: 100, Rate: -1}),                        // ずっと減少
		"1013": makeDailyData("1013", previousDate, 1000, CloseTestData{Num: 50}, CloseTestData{Num: 50, Rate: -1}), // 前半増加、後半減少
		"1014": makeDailyData("1014", previousDate, 1000, CloseTestData{Num: 50, Rate: -1}, CloseTestData{Num: 50}), // 前半減少、後半増加
		"1015": makeDailyData("1015", previousDate, 1000, CloseTestData{Num: 80}, CloseTestData{Num: 10, Rate: -1}, CloseTestData{Num: 9}, CloseTestData{Num: 1, Rate: 100}),
		"1016": makeDailyData("1016", previousDate, 1000, CloseTestData{Num: 80}, CloseTestData{Num: 10, Rate: 1}, CloseTestData{Num: 9, Rate: -1}, CloseTestData{Num: 1, Rate: -100}),
		"1017": makeDailyData("1017", previousDate, 1000, CloseTestData{Num: 80}, CloseTestData{Num: 10}, CloseTestData{Num: 10, Rate: -1}),
	}

	var inputsDaily, inputsMovingAvg [][]string
	// 一気にinsertするため一つにまとめる
	for _, v := range dailyData {
		inputsDaily = append(inputsDaily, v...)
		inputsMovingAvg = append(inputsMovingAvg, makeMovingAvgDataFromDailyTestData(v)...)
	}
	// insert daily & movingavg test data to DB
	if err := db.InsertDB("daily", inputsDaily); err != nil {
		t.Error(err)
	}
	if err := db.InsertDB("movingavg", inputsMovingAvg); err != nil {
		t.Error(err)
	}

	tests := map[string]struct {
		targetDate string
		wantCode   []string
	}{
		"success": {
			targetDate: "2020/03/05",
			wantCode:   []string{"1015", "1011", "1014", "1013", "1012", "1017", "1016"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			g := CalculateGrowthTrend{
				db:              db,
				sheet:           trendSheet,
				calcConcurrency: 3,
				targetDate:      tc.targetDate,
			}
			var codes []string
			for code := range dailyData {
				codes = append(codes, code)
			}

			// if err := g.growthTrend(ctx, codes); err != nil {
			trends, err := g.gatherAllTrends(ctx, codes)
			if err != nil {
				t.Error(err)
			}

			trendData := g.makeTrendDataForSheet(trends)
			// 以下の形になるはず
			// [code trend increaseRate crossMovingAvg5 20200305]
			// [1015 longTermAdvance 1.093 true]
			// [1011 longTermAdvance 1.001 false]
			// [1014 shortTermAdvance 1.001 false]
			// [1013 shortTermDecline 0.999 false]
			// [1012 longTermDecline 0.9989 false]
			// [1017 non 0.9991 false]
			// [1016 non 0.9075 true]

			var gotCodes []string
			for i, v := range trendData {
				t.Log(v)
				if i == 0 {
					continue
				}
				gotCodes = append(gotCodes, v[0])
			}
			if !reflect.DeepEqual(gotCodes, tc.wantCode) {
				t.Errorf("gotCodes: %v, wantCodes: %v", gotCodes, tc.wantCode)
			}
		})
	}
}
