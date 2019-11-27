package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

var (
	html = `
	<!doctype html>
	<html>
	<head>
	  <title>Stock price</title>
	</head>
	<body>
	
	<div class="m-tableType01 a-mb12">
	   <div class="m-tableType01_table">
		  <table class="w668">
			 <thead>
				<tr>
				   <th class="a-taC">日付</th>
				   <th class="a-taC">始値</th>
				   <th class="a-taC">高値</th>
				   <th class="a-taC">安値</th>
				   <th class="a-taC">終値</th>
				   <th class="a-taC">売買高</th>
				   <th class="a-taC">修正後終値</th>
				</tr>
			 </thead>
				<tbody>
				<tr>
								<th class="a-taC">
									5/16（木）</th>
								<td class="a-taR a-wordBreakAll">4,826</td>
								<td class="a-taR a-wordBreakAll">4,866</td>
								<td class="a-taR a-wordBreakAll">4,790</td>
								<td class="a-taR a-wordBreakAll">4,800</td>
								<td class="a-taR a-wordBreakAll">5,440,600</td>
								<td class="a-taR a-wordBreakAll">4,800.0</td>
							</tr>
						<tr>
								<th class="a-taC">
									5/15（水）</th>
								<td class="a-taR a-wordBreakAll">4,841</td>
								<td class="a-taR a-wordBreakAll">4,854</td>
								<td class="a-taR a-wordBreakAll">4,781</td>
								<td class="a-taR a-wordBreakAll">4,854</td>
								<td class="a-taR a-wordBreakAll">5,077,200</td>
								<td class="a-taR a-wordBreakAll">4,854.0</td>
							</tr>
						<tr>
								<th class="a-taC">
									5/14（火）</th>
								<td class="a-taR a-wordBreakAll">4,780</td>
								<td class="a-taR a-wordBreakAll">4,873</td>
								<td class="a-taR a-wordBreakAll">4,775</td>
								<td class="a-taR a-wordBreakAll">4,870</td>
								<td class="a-taR a-wordBreakAll">7,363,600</td>
								<td class="a-taR a-wordBreakAll">4,870.0</td>
							</tr>
						</tbody>
		  </table>
	   </div>
	</div>
	
	<!----></div>
	
	</body>
	</html>
`
)

func TestScrapeDailyStockPrices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, html)
	}))
	defer ts.Close()

	prices, err := scrapeDailyStockPrices(ctx, "9432", ts.URL+"/")
	if err != nil {
		t.Error(err)
	}

	want := [][]string{
		[]string{"2019/05/16", "4826", "4866", "4790", "4800", "5440600", "4800.0"},
		[]string{"2019/05/15", "4841", "4854", "4781", "4854", "5077200", "4854.0"},
		[]string{"2019/05/14", "4780", "4873", "4775", "4870", "7363600", "4870.0"},
	}
	if !reflect.DeepEqual(prices, want) {
		t.Errorf("got %v, want %v", prices, want)
	}
}

func TestScrapeDailyStockPricesEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	}))
	defer ts.Close()

	wantErr := fmt.Errorf("failed to fetch datePrices(no data) for code '9999'")
	_, err := scrapeDailyStockPrices(ctx, "9999", ts.URL+"/")
	if fmt.Sprintf("%v", err) != fmt.Sprintf("%v", wantErr) {
		t.Errorf("got err: %v want err: %v", err, wantErr)
	}
}

func TestFormatDate(t *testing.T) {
	// テスト起動時刻を 2019/2/1と設定
	now := time.Date(2019, 2, 1, 0, 0, 0, 0, time.Local)

	cases := []struct {
		name    string
		in      string
		out     string
		wantErr bool
	}{
		{"thismonth", "2/1", "2019/02/01", false},
		{"thismonth", "2/11", "2019/02/11", false},
		{"previous", "1/11", "2019/01/11", false},
		{"over", "3/11", "2018/03/11", false},
		{"over2", "12/1", "2018/12/01", false},
		{"no/", "121", "", true},
		{"notint", "aa/1", "", true},
		{"notint2", "2/b", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatDate(now, tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %t", err, tt.wantErr)
			}
			if got != tt.out {
				t.Errorf("got %s, want %s", got, tt.out)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		out     string
		wantErr bool
	}{
		{"normal", "123", "123", false},
		{"has,", "1,234", "1234", false},
		{"decimal", "1234.5", "1234.5", false},
		{"decimal2", "1234.00", "1234.00", false},
		{"not_numeral_-", "-", "", true},
		{"not_numeral_a", "a", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPrice(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %t", err, tt.wantErr)
			}
			if got != tt.out {
				t.Errorf("got %s, want %s", got, tt.out)
			}
		})
	}
}

// TODO: 以下、テストデータを作成しようとした残骸。やる気があったらこれを使う
// var (
// 	header = `<!doctype html>
// 	<html>
// 	<head>
// 	  <title>Stock price</title>
// 	</head>
// 	<body>

// 	<div class="m-tableType01 a-mb12">
// 	   <div class="m-tableType01_table">
// 		  <table class="w668">
// 			 <thead>
// 				<tr>
// 				   <th class="a-taC">日付</th>
// 				   <th class="a-taC">始値</th>
// 				   <th class="a-taC">高値</th>
// 				   <th class="a-taC">安値</th>
// 				   <th class="a-taC">終値</th>
// 				   <th class="a-taC">売買高</th>
// 				   <th class="a-taC">修正後終値</th>
// 				</tr>
// 			 </thead>
// 				<tbody>
// 	`
// 	footer = `
// 			</tbody>
// 		</table>
// 	</div>
// </div>

// <!----></div>

// </body>
// </html>`
// )

// type stockpriceList struct {
// 	List []stockprice
// }

// func (l stockpriceList) toHTMLDoc() string {
// 	fmt.Println(len(l.List))
// 	return ""
// }

// type stockprice struct {
// 	Date     string
// 	Open     string
// 	High     string
// 	Low      string
// 	Close    string
// 	Turnover string
// 	Modified string
// }

// func (s *stockprice) toHTML() string {
// 	return fmt.Sprintf(`<tr>
// 	<th class="a-taC">
// 		%s</th>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// 	<td class="a-taR a-wordBreakAll">%s</td>
// </tr>`, s.Date, s.Open, s.High, s.Low, s.Close, s.Turnover, s.Modified)
// }

// // func (s *stockprice) toHTML() string {
// // 	return fmt.Sprintf(`%s<tr>
// // 	<th class="a-taC">
// // 		%s</th>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // 	<td class="a-taR a-wordBreakAll">%s</td>
// // </tr>%s`, header, s.Date, s.Open, s.High, s.Low, s.Close, s.Turnover, s.Modified, footer)
// // }

// func TestScrapeDailyStockPrices(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	cases := []struct {
// 		name    string
// 		doc     stockprice
// 		want    [][]string
// 		wantErr bool
// 	}{
// 		{"1001",
// 			stockprice{"4/4（木）", "110", "150", "100", "120", "190", "120.1"},
// 			[][]string{[]string{"2019/04/04", "110", "150", "100", "120", "190", "120.1"}},
// 			false},
// 	}
// 	for _, tt := range cases {
// 		t.Run(tt.name, func(t *testing.T) {
// 			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 				//stockpriceList{tt.doc}
// 				fmt.Fprintf(w, tt.doc.toHTML())
// 			}))
// 			defer ts.Close()
// 			prices, err := scrapeDailyStockPrices(ctx, tt.name, ts.URL+"/")
// 			if (err != nil) != tt.wantErr {
// 				t.Fatalf("got error %v, wantErr %t", err, tt.wantErr)
// 			}
// 			if !reflect.DeepEqual(prices, tt.want) {
// 				t.Errorf("got %s, want %s", prices, tt.want)
// 			}
// 		})
// 	}
// }

// func addComma(s string) string {
// 	var newStr []string

// 	// 小数部分を格納
// 	numDeci := strings.Split(s, ".")
// 	if len(numDeci) == 2 {
// 		newStr = append(newStr, "."+numDeci[1])
// 		s = numDeci[0]
// 	}
// 	// 以下Sliceに入れてあとで結合
// 	for {
// 		start := len(s) - 3
// 		if start <= 0 {
// 			start = 0
// 			// 前に追加する
// 			newStr = append([]string{s[start:]}, newStr...)
// 			break
// 		}
// 		//fmt.Println(s[start:])
// 		// カンマ付きで前に追加する
// 		newStr = append([]string{"," + s[start:]}, newStr...)
// 		s = s[:start]
// 	}
// 	return strings.Join(newStr, "")
// }
