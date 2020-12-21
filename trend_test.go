// +build !integration

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
	"golang.org/x/sync/errgroup"
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
			},
			longTermThresholdDays: 2,
			wantCode:              []string{"1015", "1011", "1020", "1014", "1023", "1019", "1017", "1018", "1022", "1013", "1021", "1012", "1016"},
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

			var inputsDaily, inputsMovingAvg, inputsTrend [][]string
			// 一気にinsertするため一つにまとめる
			for code, dateCloses := range tc.dailyData {
				inputsDaily = append(inputsDaily, convertDateClosesToStringSlice(code, dateCloses)...)

				dms := makeDateMovingAvgFromDailyData(dateCloses)
				inputsMovingAvg = append(inputsMovingAvg, convertDateMovingAvgToStringSlice(code, dms)...)

				tts := makePreviousTrendTables(code, targetDateStr, dms, dateCloses, tc.longTermThresholdDays)
				inputsTrend = append(inputsTrend, convertTrendTablesToStringSlice(tts)...)
			}

			// insert daily & movingavg & trend test data to DB
			if err := insertTestDataToDB(db, inputsDaily, inputsMovingAvg, inputsTrend); err != nil {
				t.Fatalf("failed to insert db: %v", err)
			}

			g := CalculateGrowthTrend{
				db:                    db,
				sheet:                 trendSheet,
				calcConcurrency:       3,
				targetDate:            targetDateStr,
				longTermThresholdDays: tc.longTermThresholdDays,
			}
			var codes []string
			for code := range tc.dailyData {
				codes = append(codes, code)
			}

			if err := g.growthTrend(ctx, codes); err != nil {
				t.Error(err)
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

func makeDateMovingAvgFromDailyData(dateCloses []DateClose) []DateMovingAvgs {
	dcs := DateCloses(dateCloses)

	moving := make(map[int]map[string]float64)
	for _, days := range targetMovingAvgs {
		moving[days] = dcs.calcMovingAvg(days)
	}

	var dateMovingAvgs []DateMovingAvgs
	for _, r := range dcs {
		d := r.Date // 日付
		dm := DateMovingAvgs{
			date: d,
			movingAvgs: MovingAvgs{
				M3:   moving[3][d],
				M5:   moving[5][d],
				M7:   moving[7][d],
				M10:  moving[10][d],
				M20:  moving[20][d],
				M60:  moving[60][d],
				M100: moving[100][d],
			},
		}
		dateMovingAvgs = append(dateMovingAvgs, dm)
	}
	return dateMovingAvgs
}

func convertDateMovingAvgToStringSlice(code string, dms []DateMovingAvgs) [][]string {
	str := func(f float64) string {
		return fmt.Sprintf("%g", f)
	}

	var ss [][]string
	for _, dm := range dms {
		d := dm.date
		ss = append(ss, []string{code, d, str(dm.movingAvgs.M3), str(dm.movingAvgs.M5), str(dm.movingAvgs.M7), str(dm.movingAvgs.M10), str(dm.movingAvgs.M20), str(dm.movingAvgs.M60), str(dm.movingAvgs.M100)})
	}
	return ss
}

// trendのテストで使う過去のTrendを作成する
// growthTrend関数は、targetDateまでの過去データがdailyとmovingavg tableに格納されている前提で、
// targetDateのtrendTableを作成するので、trend tableにはtargetDate前日分までが格納される必要があることに注意
// (trend table はdailyなどと比べてtargetDate当日分のデータが入らない状態でgrowthTrend関数が動き, targetDateのTrendはその時点で格納される)
func makePreviousTrendTables(code string, targetDate string, dms []DateMovingAvgs, dateCloses []DateClose, longTermThresholdDays int) []TrendTable {
	var trendTables []TrendTable
	pastTrends := []Trend{}

	for i := len(dms) - 1; i >= 0; i-- { // 日付の古い順
		dm := dms[i]
		date := dm.date

		if date == targetDate {
			continue
		}

		ms := dm.movingAvgs
		tm := TrendMovingAvgs{
			M5:   ms.M5,
			M20:  ms.M20,
			M60:  ms.M60,
			M100: ms.M100,
		}

		closes := makeClosesForTrendTable(date, dateCloses)

		reversedPastTrends := makeReversePastTrends(pastTrends, longTermThresholdDays)
		latestTrendTable := makeTrendTable(code, date, tm, reversedPastTrends, closes, longTermThresholdDays)
		trendTables = append(trendTables, latestTrendTable)

		pastTrends = append(pastTrends, latestTrendTable.trend)
	}
	return trendTables
}

func makeClosesForTrendTable(date string, dateCloses []DateClose) []float64 {
	var closes []float64
	for _, dateClose := range dateCloses {
		if date < dateClose.Date {
			continue
		}
		// fmt.Println("hgeeeeeee", date, dateClose.Date, dateClose.Close)
		closes = append(closes, dateClose.Close)
		if len(closes) == 12 { // 直近の12件とったらおしまい(continuationDaysを最大１１まで取るため)
			break
		}
	}
	return closes
}

// このpastTrendsは日付の古い順に入っているので、逆順のSliceを作る
// longTermThresholdDaysに達したら打ち切る
func makeReversePastTrends(pastTrends []Trend, longTermThresholdDays int) []Trend {
	tmpTrends := make([]Trend, 0, len(pastTrends))
	for j := len(pastTrends) - 1; j >= 0; j-- {
		tmpTrends = append(tmpTrends, pastTrends[j])
		if len(tmpTrends) >= longTermThresholdDays {
			return tmpTrends
		}
	}
	return tmpTrends
}

func convertTrendTablesToStringSlice(tts []TrendTable) [][]string {
	var ss [][]string
	for _, tt := range tts {
		ss = append(ss, trendTableToStringForDB(tt))
	}
	return ss
}

func insertTestDataToDB(db database.DB, inputsDaily, inputsMovingAvg, inputsTrend [][]string) error {
	start := now()
	defer func() {
		fmt.Println("insert db duration:", time.Since(start))
	}()

	tableAndData := map[string][][]string{
		"daily":     inputsDaily,
		"movingavg": inputsMovingAvg,
		"trend":     inputsTrend,
	}

	// 時間短縮のためDatabeseへのテストデータの格納を並行処理で行う
	// 個別にInsertするより1.5秒ちょっと速い
	eg := errgroup.Group{}
	for table, data := range tableAndData {
		table := table
		data := data
		eg.Go(func() error {
			if err := db.InsertDB(table, data); err != nil {
				return fmt.Errorf("failed to insert %s: %v", table, err)
			}
			return nil
		})
	}
	return eg.Wait()
}

func TestCalcContinuationDays(t *testing.T) {
	tests := map[string]struct {
		closes []float64
		want   int
	}{
		"0": {
			closes: []float64{},
			want:   0,
		},
		"0-2": {
			closes: nil,
			want:   0,
		},
		"0-3": {
			closes: []float64{100},
			want:   0,
		},
		"0-4": { // 前と同じ値なら0
			closes: []float64{100, 100},
			want:   0,
		},
		"up-1": {
			closes: []float64{100, 99},
			want:   1,
		},
		"up-1-2": {
			closes: []float64{100, 99, 99},
			want:   1,
		},
		"up-1-3": {
			closes: []float64{100, 99, 100},
			want:   1,
		},
		"up-2": {
			closes: []float64{100, 99, 98},
			want:   2,
		},
		"up-2-2": {
			closes: []float64{100, 99, 98, 98},
			want:   2,
		},
		"up-2-3": {
			closes: []float64{100, 99, 98, 99},
			want:   2,
		},
		"up-3": {
			closes: []float64{100, 99, 98, 97},
			want:   3,
		},
		"up-4": {
			closes: []float64{100, 99, 98, 97, 96},
			want:   4,
		},
		"up-5": {
			closes: []float64{100, 99, 98, 97, 96, 95},
			want:   5,
		},
		"up-6": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94},
			want:   6,
		},
		"up-7": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93},
			want:   7,
		},
		"up-8": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92},
			want:   8,
		},
		"up-9": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91},
			want:   9,
		},
		"up-10": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90},
			want:   10,
		},
		"up-11": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89},
			want:   11,
		},
		"up-12": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88},
			want:   11, // 11より上はない
		},
		"down-1": {
			closes: []float64{100, 101},
			want:   1,
		},
		"down-1-2": {
			closes: []float64{100, 101, 101},
			want:   1,
		},
		"down-1-3": {
			closes: []float64{100, 101, 100},
			want:   1,
		},
		"down-2": {
			closes: []float64{100, 101, 102},
			want:   2,
		},
		"down-2-2": {
			closes: []float64{100, 101, 102, 101},
			want:   2,
		},
		"down-2-3": {
			closes: []float64{100, 101, 102, 102},
			want:   2,
		},
		"down-3": {
			closes: []float64{100, 101, 102, 103},
			want:   3,
		},
		"down-4": {
			closes: []float64{100, 101, 102, 103, 104},
			want:   4,
		},
		"down-5": {
			closes: []float64{100, 101, 102, 103, 104, 105},
			want:   5,
		},
		"down-6": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106},
			want:   6,
		},
		"down-7": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107},
			want:   7,
		},
		"down-8": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108},
			want:   8,
		},
		"down-9": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109},
			want:   9,
		},
		"down-10": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110},
			want:   10,
		},
		"down-11": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111},
			want:   11,
		},
		"down-12": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112},
			want:   11,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := calcContinuationDays(tc.closes); got != tc.want {
				t.Errorf("got: %d, want: %d", got, tc.want)
			}
		})
	}
}
