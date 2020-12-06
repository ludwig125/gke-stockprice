package main

import (
	"context"
	"fmt"
	"log"
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
	log.Println("gather trends...")
	trends, err := g.gatherAllTrends(ctx, codes)
	if err != nil {
		return fmt.Errorf("failed to gatherAllTrends: %w", err)
	}
	log.Println("gathered trendInfo successfully")

	trendData := g.makeTrendDataForSheet(trends)
	log.Println("try to print trend to sheet")
	if err := g.sheet.Update(trendData); err != nil {
		return fmt.Errorf("failed to print trend data to sheet: %w", err)
	}
	log.Println("printGrowthTrendsToSheet successfully")
	return nil
}

func (g CalculateGrowthTrend) gatherAllTrends(ctx context.Context, codes []string) ([]trendInfo, error) {
	eg, ctx := errgroup.WithContext(ctx)
	trendCh := make(chan trendInfo, len(codes))

	sem := make(chan struct{}, g.calcConcurrency)
	defer close(sem)
	for _, code := range codes {
		select {
		case <-ctx.Done():
			break
		default:
		}
		sem <- struct{}{}

		code := code
		eg.Go(func() error {
			defer func() { <-sem }()
			trend, err := g.getTrend(code)
			if err != nil {
				return fmt.Errorf("failed to get getTrend. code: %s, err:%w", code, err)
			}
			trendCh <- trend
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to eg.Wait: %w", err)
	}
	close(trendCh)

	var trends []trendInfo
	for t := range trendCh {
		trends = append(trends, t)
	}
	return trends, nil
}

func (g CalculateGrowthTrend) makeTrendDataForSheet(trends []trendInfo) [][]string {
	// increaseRate順にソート
	sort.SliceStable(trends, func(i, j int) bool {
		return trends[i].previousTrend.increaseRate > trends[j].previousTrend.increaseRate
	})

	// increaseRate順をなるべく保ちつつ、trend順にソート
	sort.SliceStable(trends, func(i, j int) bool { return trends[i].longTrend.trend > trends[j].longTrend.trend })

	var trendData [][]string
	first := 0
	for _, t := range trends {
		if first == 0 { // spreadsheetの最初の行にはカラム名を記載する
			// 日付はみんな同じなので最初の行だけに出力させる。カラム名の一番後続につける
			date := strings.Replace(t.date, "/", "", -1) // 日付に含まれるスラッシュを削る
			topLine := append(t.ColumnName(), date)
			trendData = append(trendData, topLine)
			first++
		}
		trendData = append(trendData, t.Slice())
	}
	return trendData
}

type trendInfo struct {
	code          string
	date          string
	longTrend     longTermTrend
	previousTrend previousTrend
}

// ColumnName returns column name trendInfo.
func (i trendInfo) ColumnName() []string {
	return []string{
		"code",
		// "date",
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
		// strings.Replace(i.date, "/", "", -1), // 日付に含まれるスラッシュを削る
		i.longTrend.trend.String(),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M5),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M20),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M60),
		// fmt.Sprintf("%g", i.longTrend.movingAvgs.M100),
		// fmt.Sprintf("%g", i.previousTrend.beforePreviousClose),
		// fmt.Sprintf("%g", i.previousTrend.previousClose),
		fmt.Sprintf("%.4g", i.previousTrend.increaseRate),
		fmt.Sprintf("%t", i.previousTrend.crossMovingAvg5),
	}
}

func (g CalculateGrowthTrend) getTrend(code string) (trendInfo, error) {
	long, err := g.getLongTermTrend(code)
	if err != nil {
		return trendInfo{}, fmt.Errorf("failed to getLongTermTrend: %w", err)
	}
	short, err := g.getPreviousTrend(code, long.movingAvgs.M5)
	if err != nil {
		return trendInfo{}, fmt.Errorf("failed to getPreviousTrend: %w", err)
	}
	return trendInfo{code: code, date: g.targetDate, longTrend: long, previousTrend: short}, nil
}

// Trend means stock price trend defined bellow
type Trend int

// 4: longTermAdvance : 5 > 20 > 60 > 100
// 3: shortTermAdvance : 5 > 20 > 60
// 2: shortTermDecline : 60 > 20 > 5
// 1: longTermDecline : 100 > 60 > 20 > 5
// 0: non : other

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

// longTermTrend has long term trend information.
type longTermTrend struct {
	trend      Trend
	movingAvgs TrendMovingAvgs
}

func (g CalculateGrowthTrend) getLongTermTrend(code string) (longTermTrend, error) {
	movingAvgs, err := g.getMovingAvgs(code, g.targetDate)
	if err != nil {
		return longTermTrend{}, fmt.Errorf("failed to getMovingAvgs: %w", err)
	}
	return longTermTrend{trend: classifyTrend(movingAvgs), movingAvgs: movingAvgs}, nil
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
	// moving5 > moving20 > moving60 > moving100の並びのときlongTermAdvance
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

// previousTrend has short term trend information.
type previousTrend struct {
	previousClose       float64
	beforePreviousClose float64
	increaseRate        float64
	crossMovingAvg5     bool
}

func (g CalculateGrowthTrend) getPreviousTrend(code string, m5 float64) (previousTrend, error) {
	closes, err := recentCloses(g.db, code, 2)
	if err != nil {
		return previousTrend{}, fmt.Errorf("failed to get recentCloses: %w", err)
	}
	p := closes[0].Close
	b := closes[1].Close
	return previousTrend{
		previousClose:       p, // 前営業日の終値
		beforePreviousClose: b, // 前々営業日の終値
		increaseRate:        p / b,
		crossMovingAvg5:     crossedMovingAvg5(p, b, m5),
	}, nil
}

func crossedMovingAvg5(p, b, m5 float64) bool {
	return isLeftGreaterThanRight(p, m5, b) ||
		isLeftGreaterThanRight(b, m5, p)
}
