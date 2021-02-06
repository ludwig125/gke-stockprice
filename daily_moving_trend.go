package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/sheet"
)

// TODO: フィールド変数にする
var maxContinuationDays = 11

// CalculateDailyMovingAvgTrend is configuration to calculate movingavg and growth trend.
type CalculateDailyMovingAvgTrend struct {
	db                    database.DB
	sheet                 sheet.Sheet
	calcConcurrency       int
	targetDate            string
	longTermThresholdDays int // longTermThresholdDaysの期間ShortTermのTrendが続いていたらLongとみなす閾値
}

// Exec calculates daily movingavg and trend, then write db and sheet.
func (c CalculateDailyMovingAvgTrend) Exec(codes []string) error {
	targetDate, err := time.Parse("2006/01/02", c.targetDate)
	if err != nil {
		return fmt.Errorf("failed to parse date: %s, %v", c.targetDate, err)
	}
	fromDate := targetDate.AddDate(0, 0, -100).Format("2006/01/02")

	config := CalcMovingTrendConfig{
		DB:             c.db,
		DailyTable:     "daily",
		MovingAvgTable: "movingavg",
		TrendTable:     "trend",
		Codes:          codes,
		FromDate:       fromDate,
		ToDate:         c.targetDate,
		MaxConcurrency: c.calcConcurrency,
		// TODO: LongTermThresholdDaysも環境変数から指定する
	}
	calc, err := NewCalcMovingTrend(config)
	if err != nil {
		return fmt.Errorf("failed to NewCalcMovingTrend: %w", err)
	}
	if err := calc.Exec(); err != nil {
		return fmt.Errorf("failed to Exec: %w", err)
	}

	// 最新のTrendをSpreadsheetに書き込む
	if err := c.writeSheet(codes, c.targetDate); err != nil {
		return fmt.Errorf("failed to writeSheet: %w", err)
	}
	return nil
}

func (c CalculateDailyMovingAvgTrend) writeSheet(codes []string, date string) error {
	codeTrendList, err := fetchTrendList(c.db, "trend", codes, date)
	if err != nil {
		return fmt.Errorf("failed to fetchTrendList: %v", err)
	}
	sheetData := makeTrendDataForSheet(convCodeTrendList(codeTrendList, date))
	log.Println("try to print trend to sheet")
	if err := c.sheet.Update(sheetData); err != nil {
		return fmt.Errorf("failed to print trend data to sheet: %w", err)
	}
	return nil
}

type codeDateTrendList struct {
	code             string
	date             string
	trend            Trend
	trendTurn        TrendTurnType    // trendが前回と比べてどちら向きに転換しているか
	growthRate       float64          // 前営業日の終値/前々営業日の終値
	crossMoving5     CrossMoving5Type // ２つの終値が５日移動平均線をどの向きにまたいでいるか
	continuationDays int              // 同じ傾向のGrowthが連続何日続くか
}

func (c codeDateTrendList) stringForSheet() []string {
	return []string{
		c.code,
		c.trend.String(),
		c.trendTurn.String(),
		fmt.Sprintf("%.4g", c.growthRate),
		c.crossMoving5.String(),
		fmt.Sprintf("%d", c.continuationDays),
	}
}

//codeDateTrendList のSliceに変換
func convCodeTrendList(t map[string]TrendList, date string) []codeDateTrendList {
	ctl := make([]codeDateTrendList, 0, len(t))
	for code, tl := range t {
		ctl = append(ctl, codeDateTrendList{
			code:             code,
			date:             date,
			trend:            tl.trend,
			trendTurn:        tl.trendTurn,
			growthRate:       tl.growthRate,
			crossMoving5:     tl.crossMoving5,
			continuationDays: tl.continuationDays,
		})
	}
	return ctl
}

// 希望のソート順にする
func makeTrendDataForSheet(trends []codeDateTrendList) [][]string {
	// growthRate順にソート
	sort.SliceStable(trends, func(i, j int) bool {
		return trends[i].growthRate > trends[j].growthRate
	})

	// growthRate順をなるべく保ちつつ、trend順にソート(stable sort)
	sort.SliceStable(trends, func(i, j int) bool { return trends[i].trend > trends[j].trend })

	var trendData [][]string
	first := 0
	for _, c := range trends {
		if first == 0 { // spreadsheetの最初の行にはカラム名を記載する
			// 日付はみんな同じなので最初の行だけに出力させる。カラム名の一番後続につける
			date := strings.Replace(c.date, "/", "", -1) // 日付に含まれるスラッシュを削る
			topLine := append(sheetColumnName(), date)
			trendData = append(trendData, topLine)
			first++
		}
		trendData = append(trendData, c.stringForSheet())
	}
	return trendData
}

func sheetColumnName() []string {
	return []string{
		"code",
		"trend",
		"trendTurn",
		"growthRate",
		"crossMoving5",
		"continuationDays",
	}
}

// // TrendTable has several types of trends.
// type TrendTable struct {
// 	code             string
// 	date             string
// 	trend            Trend
// 	trendTurn        TrendTurnType    // trendが前回と比べてどちら向きに転換しているか
// 	growthRate       float64          // 前営業日の終値/前々営業日の終値
// 	crossMoving5     CrossMoving5Type // ２つの終値が５日移動平均線をどの向きにまたいでいるか
// 	continuationDays int              // 同じ傾向のGrowthが連続何日続くか
// }

// func (g CalculateDailyMovingAvgTrend) getTrendTable(code string) (TrendTable, error) {
// 	movings, pastTrends, closes, err := g.fetchTrendData(code)
// 	if err != nil {
// 		return TrendTable{}, fmt.Errorf("failed to fetchTrendData: %w", err)
// 	}
// 	return makeTrendTable(code, g.targetDate, movings, pastTrends, closes, g.longTermThresholdDays), nil
// }

// func makeTrendTable(code, targetDate string, movings TrendMovingAvgs, pastTrends []Trend, closes []float64, longTermThresholdDays int) TrendTable {
// 	trend := classifyTrend(movings, pastTrends, longTermThresholdDays)
// 	return TrendTable{
// 		code:             code,
// 		date:             targetDate,
// 		trend:            trend,
// 		trendTurn:        trendTurnType(trend, pastTrends),
// 		growthRate:       latestGrowthRate(closes),
// 		crossMoving5:     crossMovingAvg5Type(closes, movings.M5),
// 		continuationDays: calcContinuationDays(closes),
// 	}
// }

// // CodeDateTrendLists maps code and DateTrendList.
// type CodeDateTrendLists map[string][]DateTrendList

// func (c CodeDateTrendLists) makeTrendDataForDB() [][]string {
// 	var trendData [][]string
// 	for code, dateTrendLists := range c {
// 		for _, dateTrendList := range dateTrendLists {
// 			trendData = append(trendData, codeDateTrendListToStringSlice(code, dateTrendList))
// 		}
// 	}
// 	return trendData
// }

// func codeDateTrendListToStringSlice(code string, dateTrendList DateTrendList) []string {
// 	trendList := dateTrendList.trendList
// 	return []string{
// 		code,
// 		dateTrendList.date,
// 		fmt.Sprintf("%d", trendList.trend),
// 		fmt.Sprintf("%d", trendList.trendTurn),
// 		fmt.Sprintf("%.4g", trendList.growthRate),
// 		fmt.Sprintf("%d", trendList.crossMoving5),
// 		fmt.Sprintf("%d", trendList.continuationDays),
// 	}
// }
