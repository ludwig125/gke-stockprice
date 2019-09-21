package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func scrapeDailyStockPrices(ctx context.Context, code string, dailyStockpriceURL string) ([][]string, error) {
	// 日足株価一ヶ月分のHDML doc取得
	doc, err := fetchWebpageDoc(ctx, code, dailyStockpriceURL, 10*time.Second)
	if err != nil {
		return nil, err
	}

	// documentから取得したデータの加工時に生じたエラーを拾う
	var docerr error

	// date と priceを取得
	var datePrices [][]string
	doc.Find(".m-tableType01_table table tbody tr").Each(func(i int, s *goquery.Selection) {
		date := s.Find(".a-taC").Text()
		re := regexp.MustCompile(`[0-9]+/[0-9]+`).Copy()
		// 日付を取得
		date = re.FindString(date)
		// スクレイピングで取得した日付に年をつけたりゼロ埋めする
		date, err := formatDate(date)
		if err != nil {
			docerr = fmt.Errorf("failed to formatDate: %w", err)
			return
		}

		datePrice := make([]string, 7)
		datePrice[0] = date //先頭に日付をつめる
		column := 1
		// 始値, 高値, 安値, 終値, 売買高, 修正後終値を順に取得
		s.Find(".a-taR").Each(func(j int, s2 *goquery.Selection) {
			// 得られた値から1,000区切りの","を取り除く
			p, err := formatPrice(s2.Text())
			if err != nil {
				docerr = fmt.Errorf("failed to formatPrice: %w", err)
				return
			}
			datePrice[column] = p // appendよりも速い
			column++
		})
		// 日付, 始値, 高値, 安値, 終値, 売買高, 修正後終値を一行ごとに格納
		datePrices = append(datePrices, datePrice)
	})
	if docerr != nil {
		return nil, fmt.Errorf("failed to format fetched data: %v", docerr)
	}
	if len(datePrices) == 0 {
		return nil, fmt.Errorf("failed to fetch datePrices(no data) for code '%s'", code)
	}
	if len(datePrices[0]) != 7 {
		// 以下の７要素を取れなかったら何かおかしい
		// 日付, 始値, 高値, 安値, 終値, 売買高, 修正後終値
		// リダイレクトされて別のページに飛ばされている可能性もある
		// 失敗した銘柄を返す
		return nil, fmt.Errorf("%s doesn't have enough elems", code)
	}

	return datePrices, nil
}

func fetchWebpageDoc(ctx context.Context, code string, dailyStockpriceURL string, timeout time.Duration) (*goquery.Document, error) {
	// requestのtimeout用に新しくctxを用意
	// 以下の方法
	// https://medium.com/congruence-labs/http-request-timeouts-in-go-for-beginners-fe6445137c90
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Request the HTML page.
	url := dailyStockpriceURL + code
	log.Printf("trying to fetch daily stockprice. code: %s", code)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to DefaultClient.Do: %v", err)
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d, status: %s, url: '%s'", res.StatusCode, res.Status, url)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to load html doc. err: %v", err)
	}
	log.Printf("fetched daily stockprice successfully. code: %s", code)
	return doc, nil
}

// 日付に年を追加する関数。現在の日付を元に前の年のものかどうか判断する
// 1/4 のような日付をゼロ埋めして01/04にする
// 例えば8/12 のような形で来たdateは 2018/08/12 にして返す
func formatDate(date string) (string, error) {
	// スクレイピングしたデータを月と日に分ける
	monthdate := strings.Split(date, "/")
	if len(monthdate) != 2 {
		return "", fmt.Errorf("failed to fetch month or date from date")
	}
	m, err := strconv.Atoi(monthdate[0])
	if err != nil {
		return "", fmt.Errorf("failed to strconv month: %w", err)
	}
	d, err := strconv.Atoi(monthdate[1])
	if err != nil {
		return "", fmt.Errorf("failed to strconv date: %w", err)
	}

	var buffer = bytes.NewBuffer(make([]byte, 0, 10))
	// スクレイピングしたデータが現在の月より先なら前の年のデータ
	// ex. 1月にスクレイピングしたデータに12月が含まれていたら前年のはず
	if m > int(now.Month()) {
		buffer.WriteString(fmt.Sprintf("%d", now.Year()-1))
	} else {
		buffer.WriteString(fmt.Sprintf("%d", now.Year()))
	}
	// あらためて年/月/日の形にして返す
	buffer.WriteString("/")
	// 2桁になるようにゼロパティング
	buffer.WriteString(fmt.Sprintf("%02d", m))
	buffer.WriteString("/")
	// 2桁になるようにゼロパティング
	buffer.WriteString(fmt.Sprintf("%02d", d))
	return buffer.String(), nil
}

// ","を取り除く関数
// 終値 5,430 -> 5430 のように修正
// 340.3のような小数点は文字列としてそのまま返す
// 数値に変換できないときはエラーを返す
func formatPrice(p string) (string, error) {
	p = strings.Replace(p, ",", "", -1)
	if _, err := strconv.ParseFloat(p, 64); err != nil {
		return "", fmt.Errorf("failed to validate price: %w", err)
	}
	return p, nil
}
