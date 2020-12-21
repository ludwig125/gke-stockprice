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
	db                    database.DB
	sheet                 sheet.Sheet
	calcConcurrency       int
	targetDate            string
	longTermThresholdDays int // longTermThresholdDaysの期間ShortTermのTrendが続いていたらLongとみなす閾値
}

func (g CalculateGrowthTrend) growthTrend(ctx context.Context, codes []string) error {
	log.Println("gather trends...")
	trends, err := g.gatherAllTrends(ctx, codes)
	if err != nil {
		return fmt.Errorf("failed to gatherAllTrends: %w", err)
	}
	log.Println("gathered trendInfo successfully")

	log.Println("try to write to database")
	trendsForDB := g.makeTrendDataForDB(trends)
	if err := g.db.InsertDB("trend", trendsForDB); err != nil {
		return fmt.Errorf("failed to insert trend table: %v", err)
	}
	log.Println("insert trend table successfully")

	trendsForSheet := g.makeTrendDataForSheet(trends)
	log.Println("try to print trend to sheet")
	if err := g.sheet.Update(trendsForSheet); err != nil {
		return fmt.Errorf("failed to print trend data to sheet: %w", err)
	}
	log.Println("printGrowthTrendsToSheet successfully")
	return nil
}

func (g CalculateGrowthTrend) gatherAllTrends(ctx context.Context, codes []string) ([]TrendTable, error) {
	eg, ctx := errgroup.WithContext(ctx)
	trendCh := make(chan TrendTable, len(codes))

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
			trend, err := g.getTrendTable(code)
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

	var trends []TrendTable
	for t := range trendCh {
		trends = append(trends, t)
	}
	return trends, nil
}

func (g CalculateGrowthTrend) makeTrendDataForDB(trends []TrendTable) [][]string {
	var trendData [][]string
	for _, t := range trends {
		trendData = append(trendData, trendTableToStringForDB(t))
	}
	return trendData
}

func trendTableToStringForDB(t TrendTable) []string {
	return []string{
		t.code,
		t.date,
		fmt.Sprintf("%d", t.trend),
		fmt.Sprintf("%d", t.trendTurn),
		fmt.Sprintf("%.4g", t.growthRate),
		fmt.Sprintf("%d", t.crossMoving5),
		fmt.Sprintf("%d", t.continuationDays),
	}
}

func (g CalculateGrowthTrend) makeTrendDataForSheet(trends []TrendTable) [][]string {
	// growthRate順にソート
	sort.SliceStable(trends, func(i, j int) bool {
		return trends[i].growthRate > trends[j].growthRate
	})

	// growthRate順をなるべく保ちつつ、trend順にソート(stable sort)
	sort.SliceStable(trends, func(i, j int) bool { return trends[i].trend > trends[j].trend })

	var trendData [][]string
	first := 0
	for _, t := range trends {
		if first == 0 { // spreadsheetの最初の行にはカラム名を記載する
			// 日付はみんな同じなので最初の行だけに出力させる。カラム名の一番後続につける
			date := strings.Replace(t.date, "/", "", -1) // 日付に含まれるスラッシュを削る
			topLine := append(sheetColumnName(), date)
			trendData = append(trendData, topLine)
			first++
		}
		trendData = append(trendData, stringForSheet(t))
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

func stringForSheet(t TrendTable) []string {
	return []string{
		t.code,
		t.trend.String(),
		t.trendTurn.String(),
		fmt.Sprintf("%.4g", t.growthRate),
		t.crossMoving5.String(),
		fmt.Sprintf("%d", t.continuationDays),
	}
}

// TrendTable has several types of trends.
type TrendTable struct {
	code             string
	date             string
	trend            Trend
	trendTurn        TrendTurnType    // trendが前回と比べてどちら向きに転換しているか
	growthRate       float64          // 前営業日の終値/前々営業日の終値
	crossMoving5     CrossMoving5Type // ２つの終値が５日移動平均線をどの向きにまたいでいるか
	continuationDays int              // 同じ傾向のGrowthが連続何日続くか
}

func (g CalculateGrowthTrend) getTrendTable(code string) (TrendTable, error) {
	movings, pastTrends, closes, err := g.fetchTrendData(code)
	if err != nil {
		return TrendTable{}, fmt.Errorf("failed to fetchTrendData: %w", err)
	}
	return makeTrendTable(code, g.targetDate, movings, pastTrends, closes, g.longTermThresholdDays), nil
}

func (g CalculateGrowthTrend) fetchTrendData(code string) (TrendMovingAvgs, []Trend, []float64, error) {
	movings, err := g.getMovingAvgs(code, g.targetDate) // movingavg table を参照する
	if err != nil {
		return TrendMovingAvgs{}, nil, nil, fmt.Errorf("failed to getMovingAvgs: %w", err)
	}

	pastTrends, err := g.getPastTrends(code) // trend tableを参照する
	if err != nil {
		return TrendMovingAvgs{}, nil, nil, fmt.Errorf("failed to getPastTrends: %w", err)
	}

	// 直近12日分のclosesを取得
	closes, err := g.getRecentCloses(code, 12) // daily table を参照する
	if err != nil {
		return TrendMovingAvgs{}, nil, nil, fmt.Errorf("failed to getRecentCloses: %w", err)
	}

	return movings, pastTrends, closes, nil
}

func makeTrendTable(code, targetDate string, movings TrendMovingAvgs, pastTrends []Trend, closes []float64, longTermThresholdDays int) TrendTable {
	trend := classifyTrend(movings, pastTrends, longTermThresholdDays)
	return TrendTable{
		code:             code,
		date:             targetDate,
		trend:            trend,
		trendTurn:        trendTurnType(trend, pastTrends),
		growthRate:       latestGrowthRate(closes),
		crossMoving5:     crossMovingAvg5Type(closes, movings.M5),
		continuationDays: calcContinuationDays(closes),
	}
}

// Trend means stock price trend defined bellow
type Trend int

// 5: longTermAdvance : 5 > 20 > 60 > 100
// 4: shortTermAdvance : 5 > 20 > 60
// 3: non : other
// 2: shortTermDecline : 60 > 20 > 5
// 1: longTermDecline : 100 > 60 > 20 > 5
// 0: unknown

const (
	unknown Trend = iota
	longTermDecline
	shortTermDecline
	non
	shortTermAdvance
	longTermAdvance
)

// constのString変換メソッド
func (t Trend) String() string {
	return [6]string{"unknown", "longTermDecline", "shortTermDecline", "non", "shortTermAdvance", "longTermAdvance"}[t]
}

// TrendMovingAvgs has movingavg 5, 20, 60, 100
type TrendMovingAvgs struct {
	M5   float64 // ５日移動平均
	M20  float64
	M60  float64
	M100 float64
}

// 銘柄コード、日付を渡すと該当のmovings structに対応するX日移動平均を返す
func (g CalculateGrowthTrend) getMovingAvgs(code, targetDate string) (TrendMovingAvgs, error) {
	q := fmt.Sprintf("SELECT moving5, moving20, moving60, moving100 FROM movingavg WHERE code = %s and date = '%s';", code, targetDate)
	ms, err := g.db.SelectDB(q)
	if err != nil {
		return TrendMovingAvgs{}, fmt.Errorf("failed to selectDB: %v", err)
	}
	if len(ms) == 0 {
		log.Printf("no selected data. query: `%s`", q)
		return TrendMovingAvgs{}, nil
		// return TrendMovingAvgs{}, fmt.Errorf("no selected data. query: `%s`", q)
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

// 一つ前の営業日のTrendを取得する
func (g CalculateGrowthTrend) getPastTrends(code string) ([]Trend, error) {
	q := fmt.Sprintf("SELECT trend FROM trend WHERE code = '%v' ORDER BY date DESC LIMIT %d;", code, g.longTermThresholdDays)
	res, err := g.db.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v", err)
	}
	if len(res) == 0 {
		log.Printf("%s: got no pastTrend", code)
		return []Trend{}, nil // 過去のTrendがない場合はエラーにはしない
	}

	if len(res) < g.longTermThresholdDays { // TODO ここ不要では
		log.Printf("%s: got not enouth pastTrend. threshold: %d", code, g.longTermThresholdDays)
		return []Trend{}, nil // 過去のTrendがない場合はエラーにはしない
	}

	var pastTrends []Trend
	for _, r := range res { // resは[ [3] [4] [4] ]のような形式で入っている
		trend := r[0]
		t, err := strconv.Atoi(trend)
		if err != nil {
			return nil, fmt.Errorf("failed to convert string trend to int: %v", err)
		}
		pastTrends = append(pastTrends, Trend(t))
	}
	return pastTrends, nil
}

// classifyTrend classify Trend by comparing movings
func classifyTrend(m TrendMovingAvgs, pastTrends []Trend, longTermThresholdDays int) Trend {
	if isLeftGreaterThanRight(m.M5, m.M20, m.M60) {
		if !isLeftGreaterThanRight(m.M60, m.M100) {
			return shortTermAdvance
		}
		if len(pastTrends) < longTermThresholdDays {
			return shortTermAdvance
		}
		// 直近のlongTermThresholdDays個分のpastTrendsがすべてshortTerm以上でなければlongTermにはしない
		if anyPastTrendLessThanTarget(pastTrends, shortTermAdvance) {
			return shortTermAdvance
		}
		// moving5 > moving20 > moving60 > moving100で、pastTrendsがすべてshortTermだったらlong
		// 直近の一定期間がshortTermでないのにlongTermになるのは不自然なので補正をかけている
		// 例：99日間増加ゼロでも最後の１日ほんの少し増加するだけで m5>m20>m60>m100が成立してしまうのでこれを考慮している
		return longTermAdvance
	}
	// 条件の厳しい順にしないとゆるい方(shortTermDecline)に先に適合してしまうので注意
	if isLeftGreaterThanRight(m.M60, m.M20, m.M5) {
		if !isLeftGreaterThanRight(m.M100, m.M60) {
			return shortTermDecline
		}
		if len(pastTrends) < longTermThresholdDays {
			return shortTermDecline
		}
		// 直近のlongTermThresholdDays個分のpastTrendsがすべてshortTerm以下でなければlongTermにはしない
		if anyPastTrendMoreThanTarget(pastTrends, shortTermDecline) {
			return shortTermDecline
		}
		return longTermDecline
	}
	return non
}

// どれか一つでもtargetTrend未満だったらtrue
func anyPastTrendLessThanTarget(pastTrends []Trend, targetTrend Trend) bool {
	for _, t := range pastTrends {
		if int(t) < int(targetTrend) {
			return true
		}
	}
	return false
}

// どれか一つでもtargetTrendを超えていたらtrue
func anyPastTrendMoreThanTarget(pastTrends []Trend, targetTrend Trend) bool {
	for _, t := range pastTrends {
		if int(t) > int(targetTrend) {
			return true
		}
	}
	return false
}

// 日付の降順で返す
func (g CalculateGrowthTrend) getRecentCloses(code string, days int) ([]float64, error) {
	if days <= 0 {
		return nil, fmt.Errorf("invalid days: %d. days should be larger than zero", days)
	}
	limitStr := fmt.Sprintf("LIMIT %d", days)
	q := fmt.Sprintf("SELECT  close FROM daily WHERE code = '%v' ORDER BY date DESC %s;", code, limitStr)
	res, err := g.db.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no selected data, query: %s", q)
	}
	var closes []float64
	for _, r := range res {
		// float64型数値に変換
		// 株価には小数点が入っていることがあるのでfloatで扱う
		f, err := strconv.ParseFloat(r[0], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat. %v", err)
		}
		closes = append(closes, f)
	}
	return closes, nil
}

// TrendTurnType is type of trend tunr.
type TrendTurnType int

// 3: upwardTurn
// 2: noTurn
// 1: downwardTurn
// 0: unknownTurn

const (
	unknownTurn TrendTurnType = iota
	downwardTurn
	noTurn
	upwardTurn
)

// constのString変換メソッド
func (t TrendTurnType) String() string {
	return [4]string{"unknownTurn", "downwardTurn", "noTurn", "upwardTurn"}[t]
}

func trendTurnType(trend Trend, pastTrends []Trend) TrendTurnType {
	if len(pastTrends) == 0 { // pastTrendsがサイズ０の時は無視
		return unknownTurn
	}
	prevTrend := pastTrends[0]
	if prevTrend == 0 { // prevTrendがunknownの時は無視
		return unknownTurn
	}
	if trend > prevTrend {
		return upwardTurn
	}
	if trend < prevTrend {
		return downwardTurn
	}
	return noTurn
}

// 直近の１日前の終値/２日前の終値の率を返す
// closesが２日分ない場合は0を返す
func latestGrowthRate(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	return closes[0] / closes[1]
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

// CrossMoving5Type is type of cross moving5.
type CrossMoving5Type int

// 3: upwardCross
// 2: noCross
// 1: downwardCross
// 0: unknownCross

const (
	unknownCross CrossMoving5Type = iota
	downwardCross
	noCross
	upwardCross
)

// constのString変換メソッド
func (c CrossMoving5Type) String() string {
	return [4]string{"unknownCross", "downwardCross", "noCross", "upwardCross"}[c]
}

func crossMovingAvg5Type(closes []float64, moving5 float64) CrossMoving5Type {
	if len(closes) < 2 {
		return noCross
	}
	prevClose := closes[0]
	beforePrevClose := closes[1]
	if isLeftGreaterThanRight(prevClose, moving5, beforePrevClose) {
		return upwardCross
	}
	if isLeftGreaterThanRight(beforePrevClose, moving5, prevClose) {
		return downwardCross
	}
	return noCross
}

func calcContinuationDays(closes []float64) int {
	if len(closes) < 2 {
		return 0
	}
	continuationDays := 0
	direction := 0
	if closes[0] > closes[1] { // 増加方向
		direction = 1
	} else if closes[0] < closes[1] { // 減少方向
		direction = -1
	} else { // 同じ値なら連続は0
		return 0
	}
	continuationDays = 1

	for i := 1; i < 11; i++ {
		evaluateRange := i + 2 // 評価対象のclose数。3から順に大きくして連続何日続いているか調べる
		if len(closes) < evaluateRange {
			return continuationDays
		}
		if direction == 1 { // 増加方向
			if closes[i] <= closes[i+1] { // 前の日と同じか減少してたらそこでおしまい
				return continuationDays
			}
		}
		if direction == -1 { // 減少方向
			if closes[i] >= closes[i+1] { // 前の日と同じか増加してたらそこでおしまい
				return continuationDays
			}
		}
		continuationDays++
	}

	// continuationDays	は最大11でおしまい
	return continuationDays
}

// TODO： 以下のような判定関数を今後用いる？
func gereralTrend(trend Trend, growthRate float64, crossMoving5 bool, prevTrend Trend, trendTurn bool) {
	// 非常に強い買い局面(上昇トレンドに転換)
	// trendがAdvance、crossMoving5がTrue、growthRateが正、trendTurnがTrueで、prevTrendがtrendより低い

	// 強い買い局面
	// trendがAdvance、crossMoving5がTrue、growthRateが正

	// 買い局面
	// trendがAdvance、crossMoving5がFalse

	// 弱い買い局面
	// trendがAdvance以外、crossMoving5がTrue、growthRateが正
	// or
	// trendTurnがTrueで、prevTrendがtrendより低い

	// 弱い売り局面
	// trendがDecline以外、crossMoving5がTrue、growthRateが負
	// or
	// trendTurnがTrueで、prevTrendがtrendより高い

	// 売り局面
	// trendがDecline、crossMoving5がFalse

	// 強い売り局面
	// trendがDecline、crossMoving5がTrue、growthRateが負

	// 非常に強い売り局面(下降トレンドに転換)
	// trendがDecline、crossMoving5がTrue、growthRateが負、trendTurnがTrueで、prevTrendがtrendより高い
}
