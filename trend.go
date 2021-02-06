package main

import (
	"fmt"
)

// CodeDateTrendLists maps code and DateTrendList.
type CodeDateTrendLists map[string][]DateTrendList

func (c CodeDateTrendLists) makeTrendDataForDB() [][]string {
	var trendData [][]string
	for code, dateTrendLists := range c {
		for _, dateTrendList := range dateTrendLists {
			trendData = append(trendData, codeDateTrendListToStringSlice(code, dateTrendList))
		}
	}
	return trendData
}

func codeDateTrendListToStringSlice(code string, dateTrendList DateTrendList) []string {
	trendList := dateTrendList.trendList
	return []string{
		code,
		dateTrendList.date,
		fmt.Sprintf("%d", trendList.trend),
		fmt.Sprintf("%d", trendList.trendTurn),
		fmt.Sprintf("%.4g", trendList.growthRate),
		fmt.Sprintf("%d", trendList.crossMoving5),
		fmt.Sprintf("%d", trendList.continuationDays),
	}
}

// DateTrendList has date and []TrendList.
type DateTrendList struct {
	date      string
	trendList TrendList
}

// TrendList has several trend information.
type TrendList struct {
	trend            Trend
	trendTurn        TrendTurnType
	growthRate       float64
	crossMoving5     CrossMoving5Type
	continuationDays int
}

func calculateTrendList(closes []float64, movings TrendMovingAvgs, pastTrends []Trend, longTermThresholdDays int) TrendList {
	trend := classifyTrend(movings, pastTrends, longTermThresholdDays)
	return TrendList{
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

// DateTrend has Date and Trend.
type DateTrend struct {
	Date  string
	Trend Trend
}

// DateTrends is list of DateTrend.
type DateTrends []DateTrend

func (d DateTrends) trends() []Trend {
	trends := make([]Trend, len(d))
	for i, v := range d {
		trends[i] = v.Trend
	}
	return trends
}

// DateTrendMovingAvg has date, trendMovingAvgs(5, 20, 60, 100).
type DateTrendMovingAvg struct {
	Date            string
	TrendMovingAvgs TrendMovingAvgs
}

// TrendMovingAvgs has movingavg 5, 20, 60, 100
type TrendMovingAvgs struct {
	M5   float64 // ５日移動平均
	M20  float64
	M60  float64
	M100 float64
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

	for i := 1; i < maxContinuationDays; i++ {
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
