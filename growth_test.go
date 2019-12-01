package main

import (
	"testing"

	"github.com/ludwig125/gke-stockprice/database"
)

func TestCalculateEachGrowthTrend(t *testing.T) {
	defer database.SetupTestDB(t)()

	db := database.NewTestDB(t)

	inputsDaily := [][]string{
		[]string{"1001", "2019/10/18", "2886", "2913", "2857", "100", "15500", "2874.0"},
		[]string{"1001", "2019/10/17", "2886", "2913", "2857", "90", "15500", "2874.0"},
		[]string{"1002", "2019/10/18", "2886", "2913", "2857", "100", "15500", "2874.0"},
		[]string{"1002", "2019/10/17", "2886", "2913", "2857", "99", "15500", "2874.0"},
		[]string{"1003", "2019/10/18", "2886", "2913", "2857", "100", "15500", "2874.0"},
		[]string{"1003", "2019/10/17", "2886", "2913", "2857", "101", "15500", "2874.0"},
		[]string{"1004", "2019/10/18", "2886", "2913", "2857", "100", "15500", "2874.0"},
		[]string{"1004", "2019/10/17", "2886", "2913", "2857", "110", "15500", "2874.0"},
		[]string{"1005", "2019/10/18", "2886", "2913", "2857", "100", "15500", "2874.0"},
		[]string{"1005", "2019/10/17", "2886", "2913", "2857", "100", "15500", "2874.0"},
	}
	if err := db.InsertDB("daily", inputsDaily); err != nil {
		t.Error(err)
	}

	inputsMovingAvg := [][]string{
		[]string{"1001", "2019/10/18", "106", "105", "104", "103", "102", "101", "100"},
		[]string{"1002", "2019/10/18", "106", "105", "104", "103", "102", "101", "101"},
		[]string{"1003", "2019/10/18", "106", "107", "108", "109", "110", "111", "111"},
		[]string{"1004", "2019/10/18", "106", "107", "108", "109", "110", "111", "112"},
		[]string{"1005", "2019/10/18", "106", "105", "104", "103", "102", "102", "102"},
	}
	if err := db.InsertDB("movingavg", inputsMovingAvg); err != nil {
		t.Error(err)
	}

	testcases := []struct {
		name      string
		code      string
		wantTrend Trend
	}{
		{
			"1001_long_term_rise",
			"1001",
			longTermAdvance,
		},
		{
			"1002_short_term_rise",
			"1002",
			shortTermAdvance,
		},
		{
			"1003_short_term_decline",
			"1003",
			shortTermDecline,
		},
		{
			"1004_long_term_decline",
			"1004",
			longTermDecline,
		},
		{
			"1005_non",
			"1005",
			non,
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			if err := calculateEachGrowthTrend(db, tt.code); err != nil {
				t.Error(err)
			}
		})
	}
}
