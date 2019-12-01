package main

import (
	"fmt"
	"log"
	"strconv"

	db "github.com/ludwig125/gke-stockprice/database"
)

func calculateEachGrowthTrend(db db.DB, code string) error {
	day, err := db.SelectDB("SELECT date FROM daily ORDER BY date DESC LIMIT 1")
	if err != nil {
		return fmt.Errorf("failed to selectDB: %v", err)
	}
	previousDay := day[0][0]
	movings, err := getMovingAvgs(db, code, previousDay)
	if err != nil {
		return fmt.Errorf("failed to getMovingAvgs: %v", err)
	}
	trend := movings.ClassifyTrend()

	closes, err := getRecentCloses(db, code, 2)
	if err != nil {
		return fmt.Errorf("failed to getRecentCloses: %v", err)
	}
	recent := RecentTwoCloses{Previous: closes[0].Close, BeforePrevious: closes[1].Close}

	trendInfo := TrendInfo{
		Code:             code,
		Date:             previousDay,
		Trend:            trend,
		MovingAvgs:       movings,
		RecentTwoCloses:  recent,
		IncreaseRate:     recent.IncreaseRate(),
		CrossMoving5Flag: recent.ClosesCrossedMoving5(movings.M5),
	}
	log.Println(trendInfo)

	return nil
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

// MovingAvgs has movingavg 5, 20, 60, 100
type MovingAvgs struct {
	M5   float64 // ５日移動平均
	M20  float64
	M60  float64
	M100 float64
}

// 銘柄コード、日付を渡すと該当のmovings structに対応するX日移動平均を返す
func getMovingAvgs(db db.DB, code string, date string) (MovingAvgs, error) {
	ms, err := db.SelectDB(fmt.Sprintf(
		"SELECT moving5, moving20, moving60, moving100 FROM movingavg WHERE code = %s and date = '%s';", code, date))
	if err != nil {
		return MovingAvgs{}, fmt.Errorf("failed to selectDB: %v", err)
	}
	if len(ms) == 0 {
		return MovingAvgs{}, fmt.Errorf("no selected data")
	}

	mf := make([]float64, 4)
	// string型の移動平均をfloat64に変換
	for i := 0; i < 4; i++ {
		f, err := strconv.ParseFloat(ms[0][i], 64)
		if err != nil {
			return MovingAvgs{}, fmt.Errorf("failed to ParseFloat: %v", err)
		}
		mf[i] = f
	}
	return MovingAvgs{mf[0], mf[1], mf[2], mf[3]}, nil
}

// ClassifyTrend classify Trend by comparing movings
func (m *MovingAvgs) ClassifyTrend() Trend {
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

// RecentTwoCloses has previous close and beforePrevious close
type RecentTwoCloses struct {
	Previous       float64 // 直近の終値
	BeforePrevious float64 // 直近のその一つ前の日の終値
}

// IncreaseRate is Previous/BeforePrevious
func (r *RecentTwoCloses) IncreaseRate() float64 {
	return r.Previous / r.BeforePrevious
}

// ClosesCrossedMoving5 check recent two closes if they crossed movingAvg5.
func (r *RecentTwoCloses) ClosesCrossedMoving5(m5 float64) bool {
	return isLeftGreaterThanRight(r.Previous, m5, r.BeforePrevious) ||
		isLeftGreaterThanRight(r.BeforePrevious, m5, r.Previous)
}

// TrendInfo has fileds to write spread sheet
type TrendInfo struct {
	Code string // 銘柄
	Date string // 直近の日付
	Trend
	MovingAvgs
	RecentTwoCloses
	IncreaseRate     float64
	CrossMoving5Flag bool
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

// // 可変長引数a, b, c...が a >= b >= cの順番のときにtrue
// func isLeftGreaterThanOrEqualToRight(params ...float64) bool {
// 	max := params[0]
// 	for m := 1; m < len(params); m++ {
// 		if max >= params[m] {
// 			max = params[m]
// 			continue
// 		}
// 		return false
// 	}
// 	return true
// }
