package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
)

// daily table からmovingavgとtrend tableを作り直す

// RestructureTablesFromDaily is struct for restructuring db.
type RestructureTablesFromDaily struct {
	DB                    database.DB
	DailyTable            string
	MovingAvgTable        string
	TrendTable            string
	Codes                 []string
	RestructureMovingavg  bool
	RestructureTrend      bool
	FromDate              string
	ToDate                string
	MaxConcurrency        int
	LongTermThresholdDays int
}

// RestructureTablesFromDailyConfig is config for RestructureTablesFromDaily.
type RestructureTablesFromDailyConfig struct {
	DB                    database.DB
	DailyTable            string
	MovingAvgTable        string
	TrendTable            string
	Codes                 []string
	RestructureMovingavg  bool
	RestructureTrend      bool
	FromDate              string
	ToDate                string
	MaxConcurrency        int
	LongTermThresholdDays int
}

// NewRestructureTablesFromDaily returns new RestructureTablesFromDaily.
func NewRestructureTablesFromDaily(r RestructureTablesFromDailyConfig) (*RestructureTablesFromDaily, error) {
	if r.DB == nil {
		return nil, errors.New("no database")
	}
	if r.DailyTable == "" {
		return nil, errors.New("no DailyTable")
	}
	if r.MovingAvgTable == "" {
		return nil, errors.New("no MovingAvgTable")
	}
	if r.TrendTable == "" {
		return nil, errors.New("no TrendTable")
	}
	if len(r.Codes) == 0 {
		return nil, errors.New("no codes")
	}
	sort.Strings(r.Codes) // codesを昇順でソート

	checkTimeValidation := func(v string) (string, error) {
		t, err := time.Parse("2006/01/02", v)
		if err != nil {
			return "", fmt.Errorf("invalid time format: %v. value: %v. Please set this format: YYYY/MM/DD", err, v)
		}
		return t.Format("2006/01/02"), nil
	}

	// デフォルトでは10日前
	fromDate := time.Now().AddDate(0, 0, -10).Format("2006/01/02")
	if r.FromDate != "" {
		t, err := checkTimeValidation(r.FromDate)
		if err != nil {
			return nil, fmt.Errorf("failed to checkTimeValidation: %v", err)
		}
		fromDate = t
	}
	toDate := time.Now().Format("2006/01/02")
	if r.ToDate != "" {
		t, err := checkTimeValidation(r.ToDate)
		if err != nil {
			return nil, fmt.Errorf("failed to checkTimeValidation: %v", err)
		}
		toDate = t
	}
	maxConcurrency := 1
	if r.MaxConcurrency > 0 {
		maxConcurrency = r.MaxConcurrency
	}
	longTermThresholdDays := 2 // TODO
	if r.LongTermThresholdDays > 0 {
		longTermThresholdDays = r.LongTermThresholdDays
	}

	return &RestructureTablesFromDaily{
		DB:                    r.DB,
		DailyTable:            r.DailyTable,
		MovingAvgTable:        r.MovingAvgTable,
		TrendTable:            r.TrendTable,
		Codes:                 r.Codes,
		RestructureMovingavg:  r.RestructureMovingavg,
		RestructureTrend:      r.RestructureTrend,
		FromDate:              fromDate,
		ToDate:                toDate,
		MaxConcurrency:        maxConcurrency,
		LongTermThresholdDays: longTermThresholdDays,
	}, nil
}

// Restructure is method to restructure tables.
func (r *RestructureTablesFromDaily) Restructure() error {
	// 複数Codeごとに処理する
	// 同時に処理する最大件数はMaxConcurrencyで与えられる

	var targetCodes []string
	for _, code := range r.Codes {
		targetCodes = append(targetCodes, code)
		// MaxConcurrencyに達したら一旦処理
		if len(targetCodes) == r.MaxConcurrency {
			if err := r.restructureForEachCodes(targetCodes); err != nil {
				return fmt.Errorf("failed to restructureForEachCodes: %v", err)
			}
			targetCodes = nil // 初期化
		}
	}
	// MaxConcurrencyに達しなかった残りを処理
	if len(targetCodes) > 0 {
		if err := r.restructureForEachCodes(targetCodes); err != nil {
			return fmt.Errorf("failed to restructureForEachCodes: %v", err)
		}
	}
	return nil
}

func (r *RestructureTablesFromDaily) restructureForEachCodes(targetCodes []string) error {
	start := time.Now()
	defer func() {
		// TODO： codeや期間をログに残す？
		log.Printf("restructureForEachCodes duration: %v, target codes: %d, from-to: %s-%s", time.Since(start), len(targetCodes), r.FromDate, r.ToDate)
	}()

	codeDateCloses, err := r.fetchCodesDateCloses(targetCodes)
	if err != nil {
		return fmt.Errorf("failed to fetchCodesDateCloses: %v", err)
	}
	cdms := calculateCodeDateMovingAvgs(codeDateCloses)
	cdts := calculateCodeDateTrend(codeDateCloses, cdms, r.LongTermThresholdDays)

	if err := r.writeMovingAndTrend(codeDateCloses, cdms, cdts); err != nil {
		return fmt.Errorf("failed to writeMovingAndTrend: %v", err)
	}
	return nil
}

// TODO: movingavg.goのrecentClosesと機能が重複している
func (r *RestructureTablesFromDaily) fetchCodesDateCloses(targetCodes []string) (map[string][]DateClose, error) {
	codes := joinCodeForWhereInStatement(targetCodes)

	fromDate := ""
	if r.FromDate != "" {
		fromDate = fmt.Sprintf("AND date >= '%s'", r.FromDate)
	}
	toDate := ""
	if r.ToDate != "" {
		toDate = fmt.Sprintf("AND date <= '%s'", r.ToDate)
	}
	q := fmt.Sprintf("SELECT code, date, close FROM %s WHERE code in (%s) %s %s ORDER BY code, date DESC;", r.DailyTable, codes, fromDate, toDate)
	res, err := r.DB.SelectDB(q)
	if err != nil {
		return nil, fmt.Errorf("failed to selectTable %v", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no selected data. query: '%s'", q)
	}

	codeDateCloses := make(map[string][]DateClose, len(r.Codes))
	var dcs []DateClose

	// 以下、複数のcodeとdate が混じったデータを処理するので、
	// currentCodeに現在扱っているcodeを格納して（以下の例だと最初は1001）、
	// あるループでcodeがcurrentCodeと異なったら（以下の例だと1002が出現したら）、
	// currentCodeを1002に入れ替えるという方法で区別して扱う
	// 例
	// 1001, 2020/1/3...
	// 1001, 2020/1/2...
	// 1001, 2020/1/1...
	// 1002, 2020/1/3...
	// 1002, 2020/1/2...
	// 1002, 2020/1/1...
	currentCode := ""
	prevClose := float64(1) // default 0だとgrowthRateの計算で0除算になってしまうので1とした
	for i, r := range res {
		code := r[0]
		if i == 0 {
			currentCode = code
		} else if currentCode != code {
			codeDateCloses[currentCode] = dcs
			dcs = nil
			currentCode = code
			prevClose = float64(1) // codeが変わったので1に戻す
		}
		date := r[1]

		close := r[2]
		var floatClose float64
		if close == "--" { // スクレイピングした時に`--`で格納されていることがあったので、この場合は一つ前の値にする
			floatClose = prevClose
			log.Printf("Warning. close is '--'. Use previous close: %v alternatively. code: %s, date: %s", prevClose, code, date)
		} else {
			// float64型数値に変換
			// closeには小数点が入っていることがあるのでfloatで扱う
			var err error
			floatClose, err = strconv.ParseFloat(close, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to ParseFloat. %v. code: %s, date: %s", err, code, date)
			}
		}
		prevClose = floatClose

		dcs = append(dcs, DateClose{Date: date, Close: floatClose})
	}
	codeDateCloses[currentCode] = dcs // 最後のcode分を格納

	if len(codeDateCloses) != len(targetCodes) {
		return nil, fmt.Errorf("unmatch codes. result codes: %d, targetCodes: %d", len(codeDateCloses), len(targetCodes))
	}

	return codeDateCloses, nil
}

func joinCodeForWhereInStatement(codes []string) string {
	cs := make([]string, len(codes))
	for i, code := range codes {
		cs[i] = fmt.Sprintf("'%s'", code)
	}
	return strings.Join(cs, ",")
}

// TODO: movingavg.goと機能が重複している?
func (r *RestructureTablesFromDaily) writeMovingAndTrend(cdcs map[string][]DateClose, cdms map[string][]DateMovingAvgs, cdts map[string][]DateTrendList) error {
	movingavgData := CodeDateMovingAvgs(cdms).Slices()
	if err := r.DB.InsertOrUpdateDB(r.MovingAvgTable, movingavgData); err != nil {
		return fmt.Errorf("failed to insert movingavg: %v", err)
	}

	trendData := CodeDateTrendLists(cdts).makeTrendDataForDB()
	if err := r.DB.InsertOrUpdateDB(r.TrendTable, trendData); err != nil {
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

	// (日付:移動平均)のMapを3, 5, 7,...ごとに格納したMap
	moving := make(map[int]map[string]float64)
	for _, days := range targetMovingAvgs {
		// moving[3]: 3日移動平均
		// moving[5]: 5日移動平均...
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
		date := dm.date

		ms := dm.movingAvgs
		tm := TrendMovingAvgs{
			M5:   ms.M5,
			M20:  ms.M20,
			M60:  ms.M60,
			M100: ms.M100,
		}

		closes := extractCloses(date, dateCloses)

		reversedPastTrends := makeReversePastTrends(pastTrends, longTermThresholdDays)
		latestTrendList := calculateTrendList(tm, reversedPastTrends, closes, longTermThresholdDays)

		dateTrendLists = append(dateTrendLists, DateTrendList{date: date, trendList: latestTrendList})

		// trendLists = append(trendLists, latestTrendList)

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
