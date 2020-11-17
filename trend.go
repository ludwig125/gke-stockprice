package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/ludwig125/gke-stockprice/sheet"
)

// CalculateGrowthTrend is configuration to calculate growth trend.
type CalculateGrowthTrend struct {
	db              database.DB
	sheet           sheet.Sheet
	calcConcurrency int
	targetDate      string
}

func (g CalculateGrowthTrend) growthTrend(ctx context.Context, codes []string) error {
	trends, err := g.gatherTrendInfo(ctx, codes)
	if err != nil {
		return fmt.Errorf("failed to get gatherTrendInfo: %w", err)
	}
	if err := g.printGrowthTrendsToSheet(ctx, trends); err != nil {
		return fmt.Errorf("failed to printGrowthTrendsToSheet: %w", err)
	}
	return nil
}

func (g CalculateGrowthTrend) gatherTrendInfo(ctx context.Context, codes []string) (<-chan trendInfo, error) {
	eg, ctx := errgroup.WithContext(ctx)
	trends := make(chan trendInfo, len(codes))

	sem := make(chan struct{}, g.calcConcurrency)
	defer close(sem)
	defer close(trends)
	for _, code := range codes {
		select {
		case <-ctx.Done():
			break
		default:
		}
		sem <- struct{}{} // チャネルに送信

		code := code
		eg.Go(func() error {
			defer func() { <-sem }()
			trend, err := g.trendInfo(code)
			if err != nil {
				return fmt.Errorf("failed to get trendInfo. code: %s, err:%w", code, err)
			}
			//log.Println("code", code, "trend:", trend)
			trends <- trend
			return nil
		})
	}

	return trends, eg.Wait()
}

func (g CalculateGrowthTrend) printGrowthTrendsToSheet(ctx context.Context, trends <-chan trendInfo) error {
	var ts []trendInfo
	for trend := range trends {
		ts = append(ts, trend)
	}
	// trend順にソート
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].longTrend.trend > ts[j].longTrend.trend })
	// increaseRate順にソート
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].shortTrend.increaseRate > ts[j].shortTrend.increaseRate })

	var ss [][]string
	first := 0
	for _, t := range ts {
		//fmt.Println("trend", trend)
		if first == 0 { // spreadsheetの最初の行にはカラム名を記載する
			ss = append(ss, t.ColumnName())
			first++
		}
		ss = append(ss, t.Slice())
	}
	if err := g.sheet.Update(ss); err != nil {
		return fmt.Errorf("failed to sheet Update: %w", err)
	}
	return nil
}

type trendInfo struct {
	code       string
	date       string
	longTrend  longTrend
	shortTrend shortTrend
}

// ColumnName returns column name trendInfo.
func (i trendInfo) ColumnName() []string {
	return []string{
		"code",
		"date",
		"trend",
		// "M5",
		// "M20",
		// "M60",
		// "M100",
		// "beforePreviousClose",
		// "previousClose",
		"increaseRate",
		"crossMovingAvg5",
	}
}

// Slices makes string slice from trendInfo.
func (i trendInfo) Slice() []string {
	return []string{
		i.code,
		strings.Replace(i.date, "/", "", -1), // 日付に含まれるスラッシュを削る
		i.longTrend.trend.String(),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M5),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M20),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M60),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M100),
		// fmt.Sprintf("%g", i.shortTrend.beforePreviousClose),
		// fmt.Sprintf("%g", i.shortTrend.previousClose),
		fmt.Sprintf("%.4g", i.shortTrend.increaseRate),
		fmt.Sprintf("%t", i.shortTrend.crossMovingAvg5),
	}
}

func (g CalculateGrowthTrend) trendInfo(code string) (trendInfo, error) {
	long, err := g.longTermTrend(code)
	if err != nil {
		return trendInfo{}, fmt.Errorf("failed to get longTermTrend: %w", err)
	}
	short, err := g.previousTrend(code, long.movingAvgs.M5)
	if err != nil {
		return trendInfo{}, fmt.Errorf("failed to get shortTermTrend: %w", err)
	}
	return trendInfo{code: code, date: g.targetDate, longTrend: long, shortTrend: short}, nil
}

// Trend means stock price trend defined bellow
type Trend int

// 4: longTermAdvance : 5 > 20 > 60 > 100
// 3: shortTermAdvance : 5 > 20 > 60
// 2: shortTermDecline : 60 > 20 > 5
// 1: longTermDecline : 100 > 60 > 20 > 5
// 0: NON : other

const (
	non Trend = iota
	longTermDecline
	shortTermDecline
	shortTermAdvance
	longTermAdvance
)

// constのString変換メソッド
func (t Trend) String() string {
	return [5]string{"non", "longTermDecline", "shortTermDecline", "shortTermAdvance", "longTermAdvance"}[t]
}

// TrendMovingAvgs has movingavg 5, 20, 60, 100
type TrendMovingAvgs struct {
	M5   float64 // ５日移動平均
	M20  float64
	M60  float64
	M100 float64
}

// longTrend has long term trend information.
type longTrend struct {
	trend      Trend
	movingAvgs TrendMovingAvgs
}

func (g CalculateGrowthTrend) longTermTrend(code string) (longTrend, error) {
	movingAvgs, err := g.getMovingAvgs(code, g.targetDate)
	if err != nil {
		return longTrend{}, fmt.Errorf("failed to getMovingAvgs: %w", err)
	}
	return longTrend{trend: classifyTrend(movingAvgs), movingAvgs: movingAvgs}, nil
}

// 銘柄コード、日付を渡すと該当のmovings structに対応するX日移動平均を返す
func (g CalculateGrowthTrend) getMovingAvgs(code string, date string) (TrendMovingAvgs, error) {
	q := fmt.Sprintf("SELECT moving5, moving20, moving60, moving100 FROM movingavg WHERE code = %s and date = '%s';", code, date)
	ms, err := g.db.SelectDB(q)
	if err != nil {
		return TrendMovingAvgs{}, fmt.Errorf("failed to selectDB: %v", err)
	}
	if len(ms) == 0 {
		return TrendMovingAvgs{}, fmt.Errorf("no selected data. query: `%s`", q)
	}

	mf := make([]float64, 4)
	// string型の移動平均をfloat64に変換
	for i, m := range ms[0] {
		f, err := strconv.ParseFloat(m, 64)
		if err != nil {
			return TrendMovingAvgs{}, fmt.Errorf("failed to ParseFloat: %v", err)
		}
		mf[i] = f
	}
	return TrendMovingAvgs{M5: mf[0], M20: mf[1], M60: mf[2], M100: mf[3]}, nil
}

// classifyTrend classify Trend by comparing movings
func classifyTrend(m TrendMovingAvgs) Trend {
	// moving5 > moving20 > moving60 > moving100の並びのときPPP
	if isLeftGreaterThanRight(m.M5, m.M20, m.M60, m.M100) {
		return longTermAdvance
	}
	if isLeftGreaterThanRight(m.M5, m.M20, m.M60) {
		return shortTermAdvance
	}
	// 条件の厳しい順にしないとゆるい方(shortTermDecline)に先に適合してしまうので注意
	if isLeftGreaterThanRight(m.M100, m.M60, m.M20, m.M5) {
		return longTermDecline
	}
	if isLeftGreaterThanRight(m.M60, m.M20, m.M5) {
		return shortTermDecline
	}
	return non
}

// 可変長引数a, b, c...が a > b > cの順番のときにtrue
func isLeftGreaterThanRight(params ...float64) bool {
	max := params[0]
	for m := 1; m < len(params); m++ {
		if max > params[m] {
			max = params[m]
			continue
		}
		return false
	}
	return true
}

// shortTrend has short term trend information.
type shortTrend struct {
	previousClose       float64
	beforePreviousClose float64
	increaseRate        float64
	crossMovingAvg5     bool
}

func (g CalculateGrowthTrend) previousTrend(code string, m5 float64) (shortTrend, error) {
	closes, err := recentCloses(g.db, code, 2)
	if err != nil {
		return shortTrend{}, fmt.Errorf("failed to get recentCloses: %w", err)
	}
	p := closes[0].Close
	b := closes[1].Close
	return shortTrend{
		previousClose:       p,
		beforePreviousClose: b,
		increaseRate:        p / b,
		crossMovingAvg5:     crossedMovingAvg5(p, b, m5),
	}, nil
}

func crossedMovingAvg5(p, b, m5 float64) bool {
	return isLeftGreaterThanRight(p, m5, b) ||
		isLeftGreaterThanRight(b, m5, p)
}
