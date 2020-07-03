// +build !integration

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/ludwig125/gke-stockprice/database"
)

func dummyServer(w http.ResponseWriter, r *http.Request) {
	k, ok := r.URL.Query()["scode"]
	if !ok || len(k[0]) < 1 {
		// scodeがクエリにないときは400を返す
		http.Error(w, "no code", http.StatusBadRequest)
		return
	}
	switch code := k[0]; code {
	case "90000": // 空を返す
		fmt.Fprintf(w, "")
	case "90001": // 400を返す
		http.Error(w, "400", http.StatusBadRequest)
	case "90002": // 500を返す
		http.Error(w, "500", http.StatusInternalServerError)
	default: // 与えられた銘柄に対応するHTMLファイルの中身を返す
		content, err := ioutil.ReadFile(fmt.Sprintf("testdata/webpage/%s.html", code))
		if err != nil {
			e := fmt.Sprintf("failed to read testfile: %v", err)
			http.Error(w, e, http.StatusInternalServerError)
		}
		fmt.Fprintf(w, string(content))
	}
}

func TestStockPrice(t *testing.T) {
	cleanup, err := database.SetupTestDB(3306)
	if err != nil {
		t.Fatalf("failed to SetupTestDB: %v", err)
	}
	defer cleanup()
	//defer database.SetupTestDB(t, 3306)()

	tests := map[string]struct {
		codes           []string
		wantFailedCodes []string
	}{
		"success": {
			codes:           []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			wantFailedCodes: nil,
		},
		"fail": {
			codes:           []string{"1802", "2587", "90000", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			wantFailedCodes: []string{"90000"},
		},
		"fail2": {
			codes:           []string{"1802", "2587", "90000", "90001", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			wantFailedCodes: []string{"90000", "90001"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ts := httptest.NewServer(http.HandlerFunc(dummyServer))
			defer ts.Close()

			db, err := database.NewTestDB()
			if err != nil {
				t.Fatalf("failed to NewTestDB: %v", err)
			}
			sp := DailyStockPrice{
				db:                 db,
				dailyStockpriceURL: ts.URL + "/",
				fetchInterval:      10 * time.Millisecond,
				fetchTimeout:       1 * time.Second,
			}

			currentTime := time.Date(2019, 12, 1, 0, 0, 0, 0, time.Local)
			failedCodes, err := sp.saveStockPrice(ctx, tc.codes, currentTime)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if len(failedCodes) != len(tc.wantFailedCodes) {
				t.Fatalf("got failedCodes num: %d, want failedCodes num: %d", len(failedCodes), len(tc.wantFailedCodes))
			}
			if failedCodes != nil {
				var fcodes []string
				for _, f := range failedCodes {
					t.Log(f.Error())
					fcodes = append(fcodes, f.code)
				}
				sort.Slice(fcodes, func(i, j int) bool { return fcodes[i] < fcodes[j] })
				if !reflect.DeepEqual(fcodes, tc.wantFailedCodes) {
					t.Fatalf("got failedCodes: %#v, want failedCodes: %v", fcodes, tc.wantFailedCodes)
				}
			}

			// tableに格納されたcodeの数を確認
			retCodes, err := sp.db.SelectDB("SELECT DISTINCT code FROM daily")
			if err != nil {
				t.Error(err)
			}
			wantCodesNum := (len(tc.codes) - len(tc.wantFailedCodes))
			if len(retCodes) != wantCodesNum {
				t.Errorf("got codes: %d, want: %d", len(retCodes), wantCodesNum)
				t.Logf("codes: %v", retCodes)
			}

			// tableに格納された総レコード数を確認
			retRecords, err := sp.db.SelectDB("SELECT COUNT(*) FROM daily")
			if err != nil {
				t.Error(err)
			}
			if retRecords[0][0] != fmt.Sprintf("%d", wantCodesNum*25) {
				t.Errorf("got records : %d, want: %d", len(retRecords), wantCodesNum*25)
				t.Logf("records: %#v", retRecords[0][0])
			}
		})
	}
}

func TestScrape(t *testing.T) {
	tests := map[string]struct {
		code    string
		want    []DatePrice
		wantErr bool
	}{
		"success": {
			code: "9432",
			want: []DatePrice{
				DatePrice{"2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"},
				DatePrice{"2019/05/15", "4841", "4854", "4781", "4854", "5077200", "4854.0"},
				DatePrice{"2019/05/14", "4780", "4873", "4775", "4870", "7363600", "4870.0"},
				DatePrice{"2019/05/13", "4688", "4803", "4666", "4775", "3612200", "4775.0"},
				DatePrice{"2019/05/10", "4713", "4757", "4683", "4745", "3974700", "4745.0"},
				DatePrice{"2019/05/09", "4705", "4733", "4672", "4715", "4122300", "4715.0"},
				DatePrice{"2019/05/08", "4716", "4758", "4674", "4705", "4500100", "4705.0"},
				DatePrice{"2019/05/07", "4616", "4752", "4595", "4752", "4785200", "4752.0"},
				DatePrice{"2019/04/26", "4565", "4641", "4551", "4616", "3671100", "4616.0"},
				DatePrice{"2019/04/25", "4585", "4627", "4585", "4614", "2501100", "4614.0"},
				DatePrice{"2019/04/24", "4614", "4642", "4545", "4554", "3151800", "4554.0"},
				DatePrice{"2019/04/23", "4567", "4596", "4553", "4584", "2411400", "4584.0"},
				DatePrice{"2019/04/22", "4550", "4599", "4528", "4575", "1807900", "4575.0"},
				DatePrice{"2019/04/19", "4651", "4652", "4578", "4586", "1785200", "4586.0"},
				DatePrice{"2019/04/18", "4674", "4697", "4656", "4670", "2676800", "4670.0"},
				DatePrice{"2019/04/17", "4733", "4733", "4619", "4638", "3239900", "4638.0"},
				DatePrice{"2019/04/16", "4721", "4734", "4665", "4688", "3572100", "4688.0"},
				DatePrice{"2019/04/15", "4640", "4663", "4604", "4618", "2466200", "4618.0"},
				DatePrice{"2019/04/12", "4605", "4611", "4565", "4577", "2439100", "4577.0"},
				DatePrice{"2019/04/11", "4590", "4616", "4558", "4605", "2565200", "4605.0"},
				DatePrice{"2019/04/10", "4550", "4590", "4534", "4562", "2457100", "4562.0"},
				DatePrice{"2019/04/09", "4591", "4598", "4545", "4578", "2847200", "4578.0"},
				DatePrice{"2019/04/08", "4653", "4663", "4582", "4600", "3098400", "4600.0"},
				DatePrice{"2019/04/05", "4706", "4707", "4666", "4673", "2012700", "4673.0"},
				DatePrice{"2019/04/04", "4711", "4716", "4678", "4685", "2028600", "4685.0"},
			},
			wantErr: false,
		},
		"empty_resp": {
			code:    "90000",
			want:    nil,
			wantErr: true,
		},
		"error_resp": {
			code:    "90002",
			want:    nil,
			wantErr: true,
		},
		"no_scode": {
			want:    nil,
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ts := httptest.NewServer(http.HandlerFunc(dummyServer))
			defer ts.Close()

			sp := DailyStockPrice{
				dailyStockpriceURL: ts.URL,
				fetchTimeout:       10 * time.Millisecond,
			}
			currentTime := time.Date(2019, 12, 1, 0, 0, 0, 0, time.Local)
			prices, err := sp.scrape(ctx, tc.code, currentTime)
			if (err != nil) != tc.wantErr {
				t.Errorf("error: %v, wantErr: %t", err, tc.wantErr)
				return
			}
			if err != nil {
				t.Log(err)
				return // エラーがある場合はこのあとの処理はしない
			}
			if !reflect.DeepEqual(prices, tc.want) {
				t.Fatalf("got %v, want %v", prices, tc.want)
			}
		})
	}
}
