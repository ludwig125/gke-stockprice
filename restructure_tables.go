package main

import (
	"errors"
	"fmt"
	"sort"
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
	config := CalcMovingTrendConfig{
		DB:             r.DB,
		DailyTable:     r.DailyTable,
		MovingAvgTable: r.MovingAvgTable,
		TrendTable:     r.TrendTable,
		Codes:          r.Codes,
		FromDate:       r.FromDate,
		ToDate:         r.ToDate,
		MaxConcurrency: r.MaxConcurrency,
		// TODO: LongTermThresholdDaysも環境変数から指定する
	}
	calc, err := NewCalcMovingTrend(config)
	if err != nil {
		return fmt.Errorf("failed to NewCalcMovingTrend: %w", err)
	}
	if err := calc.Exec(); err != nil {
		return fmt.Errorf("failed to Exec: %w", err)
	}
	return nil
}
