package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
	"github.com/pkg/errors"
)

// daily table からmovingavgとtrend を計算してDBに格納する

// CalcMovingTrend is struct.
type CalcMovingTrend struct {
	DB                    database.DB
	DailyTable            string
	MovingAvgTable        string
	TrendTable            string
	Codes                 []string
	FromDate              string
	ToDate                string
	MaxConcurrency        int
	LongTermThresholdDays int
	// RestructureMovingavg  bool
	// RestructureTrend      bool
}

// CalcMovingTrendConfig is config for CalcMovingTrend.
type CalcMovingTrendConfig struct {
	DB                    database.DB
	DailyTable            string
	MovingAvgTable        string
	TrendTable            string
	Codes                 []string
	FromDate              string
	ToDate                string
	MaxConcurrency        int
	LongTermThresholdDays int
	// RestructureMovingavg  bool
	// RestructureTrend      bool
}

// NewCalcMovingTrend returns new CalcMovingTrend.
func NewCalcMovingTrend(c CalcMovingTrendConfig) (*CalcMovingTrend, error) {
	if c.DB == nil {
		return nil, errors.New("no database")
	}
	if c.DailyTable == "" {
		return nil, errors.New("no DailyTable")
	}
	if c.MovingAvgTable == "" {
		return nil, errors.New("no MovingAvgTable")
	}
	if c.TrendTable == "" {
		return nil, errors.New("no TrendTable")
	}
	if len(c.Codes) == 0 {
		return nil, errors.New("no codes")
	}
	sort.Strings(c.Codes) // codesを昇順でソート

	checkTimeValidation := func(v string) (string, error) {
		t, err := time.Parse("2006/01/02", v)
		if err != nil {
			return "", fmt.Errorf("invalid time format: %v. value: %v. Please set this format: YYYY/MM/DD", err, v)
		}
		return t.Format("2006/01/02"), nil
	}

	// デフォルトでは10日前
	fromDate := time.Now().AddDate(0, 0, -10).Format("2006/01/02")
	if c.FromDate != "" {
		t, err := checkTimeValidation(c.FromDate)
		if err != nil {
			return nil, fmt.Errorf("failed to checkTimeValidation: %v", err)
		}
		fromDate = t
	}
	toDate := time.Now().Format("2006/01/02")
	if c.ToDate != "" {
		t, err := checkTimeValidation(c.ToDate)
		if err != nil {
			return nil, fmt.Errorf("failed to checkTimeValidation: %v", err)
		}
		toDate = t
	}
	maxConcurrency := 1
	if c.MaxConcurrency > 0 {
		maxConcurrency = c.MaxConcurrency
	}
	longTermThresholdDays := 2 // TODO
	if c.LongTermThresholdDays > 0 {
		longTermThresholdDays = c.LongTermThresholdDays
	}

	return &CalcMovingTrend{
		DB:                    c.DB,
		DailyTable:            c.DailyTable,
		MovingAvgTable:        c.MovingAvgTable,
		TrendTable:            c.TrendTable,
		Codes:                 c.Codes,
		FromDate:              fromDate,
		ToDate:                toDate,
		MaxConcurrency:        maxConcurrency,
		LongTermThresholdDays: longTermThresholdDays,
		// RestructureMovingavg:  c.RestructureMovingavg,
		// RestructureTrend:      c.RestructureTrend,
	}, nil
}

// Exec is method.
func (c CalcMovingTrend) Exec() error {
	// 複数Codeごとに処理する
	// 同時に処理する最大件数はMaxConcurrencyで与えられる

	log.Printf("calcForEachCode from-to: %s-%s", c.FromDate, c.ToDate)

	var targetCodes []string
	for _, code := range c.Codes {
		targetCodes = append(targetCodes, code)
		// MaxConcurrencyに達したら一旦処理
		if len(targetCodes) == c.MaxConcurrency {
			if err := c.calcForEachCode(targetCodes); err != nil {
				return fmt.Errorf("failed to calcForEachCode: %v", err)
			}
			targetCodes = nil // 初期化
		}
	}
	// MaxConcurrencyに達しなかった残りを処理
	if len(targetCodes) > 0 {
		if err := c.calcForEachCode(targetCodes); err != nil {
			return fmt.Errorf("failed to calcForEachCode: %v", err)
		}
	}
	return nil
}

func (c CalcMovingTrend) calcForEachCode(targetCodes []string) error {
	start := time.Now()
	defer func() {
		// TODO： codeや期間をログに残す？
		log.Printf("calcForEachCode duration: %v, target codes num: %d", time.Since(start), len(targetCodes))
	}()

	codeDateCloses, err := c.fetchCodesDateCloses(targetCodes)
	if err != nil {
		return fmt.Errorf("failed to fetchCodesDateCloses: %v", err)
	}
	cdms := calculateCodeDateMovingAvgs(codeDateCloses)
	cdts := calculateCodeDateTrend(codeDateCloses, cdms, c.LongTermThresholdDays)

	if err := c.writeMovingAndTrend(codeDateCloses, cdms, cdts); err != nil {
		return fmt.Errorf("failed to writeMovingAndTrend: %v", err)
	}
	log.Printf("write moving and trend successfully, code: %v", targetCodes)
	return nil
}

func (c CalcMovingTrend) fetchCodesDateCloses(targetCodes []string) (map[string][]DateClose, error) {
	fromDate := ""
	if c.FromDate != "" {
		fromDate = fmt.Sprintf("AND date >= '%s'", c.FromDate)
	}
	toDate := ""
	if c.ToDate != "" {
		toDate = fmt.Sprintf("AND date <= '%s'", c.ToDate)
	}
	return fetchCodesDateCloses(c.DB, c.DailyTable, targetCodes, fromDate, toDate, "")
}

func (c CalcMovingTrend) writeMovingAndTrend(cdcs map[string][]DateClose, cdms map[string][]DateMovingAvgs, cdts map[string][]DateTrendList) error {
	movingavgData := CodeDateMovingAvgs(cdms).Slices()
	if err := c.DB.InsertOrUpdateDB(c.MovingAvgTable, movingavgData); err != nil {
		return fmt.Errorf("failed to insert movingavg: %v", err)
	}

	trendData := CodeDateTrendLists(cdts).makeTrendDataForDB()
	if err := c.DB.InsertOrUpdateDB(c.TrendTable, trendData); err != nil {
		return fmt.Errorf("failed to insert trend: %v", err)
	}
	return nil
}

func calculateCodeDateMovingAvgs(codeDateCloses map[string][]DateClose) map[string][]DateMovingAvgs {
	cdms := make(map[string][]DateMovingAvgs, len(codeDateCloses))
	for code, dateCloses := range codeDateCloses {
		dm := calculateMovingAvg(dateCloses)
		cdms[code] = dm
	}

	return cdms
}

// TODO: movingavg.go と被っている
func calculateMovingAvg(dateCloses []DateClose) []DateMovingAvgs {
	dcs := DateCloses(dateCloses)

	// 取得対象の移動平均
	targetMovingAvgs := []int{3, 5, 7, 10, 20, 60, 100}

	// (日付:移動平均)のMapを3, 5, 7,...ごとに格納したMap
	moving := make(map[int]map[string]float64)
	for _, days := range targetMovingAvgs {
		// moving[3]: 3日移動平均
		// moving[5]: 5日移動平均...
		moving[days] = dcs.calcMovingAvg(days)
	}
	var dateMovingAvgs []DateMovingAvgs
	for _, c := range dcs {
		d := c.Date // 日付
		dm := DateMovingAvgs{
			Date: d,
			MovingAvgs: MovingAvgs{
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

func calculateCodeDateTrend(codeDateCloses map[string][]DateClose, codeDateMovingAvgs map[string][]DateMovingAvgs, longTermThresholdDays int) map[string][]DateTrendList {
	cdts := make(map[string][]DateTrendList, len(codeDateCloses))
	for code, dateCloses := range codeDateCloses {
		dt := calculateTrend(dateCloses, codeDateMovingAvgs[code], longTermThresholdDays)
		cdts[code] = dt
	}

	return cdts
}

func calculateTrend(dateCloses []DateClose, dateMovingAvgs []DateMovingAvgs, longTermThresholdDays int) []DateTrendList {
	dateTrendLists := make([]DateTrendList, 0, len(dateCloses))
	pastTrends := []Trend{}

	for i := len(dateMovingAvgs) - 1; i >= 0; i-- { // 日付の古い順
		dm := dateMovingAvgs[i]
		date := dm.Date

		ms := dm.MovingAvgs
		tm := TrendMovingAvgs{
			M5:   ms.M5,
			M20:  ms.M20,
			M60:  ms.M60,
			M100: ms.M100,
		}

		closes := extractCloses(date, dateCloses)

		reversedPastTrends := makeReversePastTrends(pastTrends, longTermThresholdDays)
		latestTrendList := calculateTrendList(closes, tm, reversedPastTrends, longTermThresholdDays)

		dateTrendLists = append(dateTrendLists, DateTrendList{date: date, trendList: latestTrendList})

		pastTrends = append(pastTrends, latestTrendList.trend)
	}

	// 最後に日付の降順にする
	for i, j := 0, len(dateTrendLists)-1; i < j; i, j = i+1, j-1 {
		dateTrendLists[i], dateTrendLists[j] = dateTrendLists[j], dateTrendLists[i]
	}

	return dateTrendLists
}

func extractCloses(date string, dateCloses []DateClose) []float64 {
	var closes []float64
	for _, dateClose := range dateCloses {
		if date < dateClose.Date {
			continue
		}
		closes = append(closes, dateClose.Close)
		if len(closes) == maxContinuationDays+1 { // 直近のmaxContinuationDays+1件とったらおしまい(continuationDaysをmaxContinuationDaysまで取るため)
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
