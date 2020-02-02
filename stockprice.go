package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/ludwig125/gke-stockprice/database"
)

// CodePrices is code and daily stockprices
type CodePrices struct {
	code   string
	prices []DatePrice
}

// Slices converts CodePrices to double string slice.
func (c CodePrices) Slices() [][]string {
	var ss [][]string
	for _, p := range c.prices {
		ss = append(ss, []string{c.code, p.date, p.open, p.high, p.low, p.close, p.turnover, p.modified})
	}
	return ss
}

// DatePrice is struct for daily stockprice.
type DatePrice struct {
	date     string
	open     string
	high     string
	low      string
	close    string
	turnover string
	modified string
}

// FailedCodes is slice of FailedCode
type FailedCodes []FailedCode

func (fs FailedCodes) Error() string {
	e := ""
	for _, f := range fs {
		e += fmt.Sprintf("%s\n", f.Error())
	}
	return e
}

// FailedCode is failed code to scrape stockprice, which has error and failed code.
type FailedCode struct {
	err  error
	code string
}

func (f FailedCode) Error() string {
	return fmt.Sprintf("code: %s, error: %v", f.code, f.err)
}

// DailyStockPrice is configuration to scrape daily stockprice page.
type DailyStockPrice struct {
	db                 database.DB
	dailyStockpriceURL string
	fetchInterval      time.Duration
	fetchTimeout       time.Duration
	currentTime        time.Time
}

func (sp DailyStockPrice) saveStockPrice(ctx context.Context, codes []string) (FailedCodes, error) {
	// scrapeで発生したerrorは全部failedCodeに入れて最後に返す
	var failedCodes FailedCodes
	var mu sync.Mutex

	// scrape先への負荷を考えて指定の単位で処理する
	t := time.NewTicker(sp.fetchInterval)
	defer t.Stop()

	eg, ctx := errgroup.WithContext(ctx)

	start := time.Now()
	defer func() {
		log.Println("saveStockPrice total time:", time.Since(start))
	}()

	for _, code := range codes {
		code := code

		select {
		case <-ctx.Done(): // ctx のcancelを受け取ったら終了
			log.Println("Stop ticker fetchStockPrice")
			return nil, ctx.Err()
		case <-t.C:

			eg.Go(func() error {
				s := time.Now()
				prices, err := sp.scrape(ctx, code)
				if err != nil {
					mu.Lock()
					failedCodes = append(failedCodes, FailedCode{err: err, code: code})
					mu.Unlock()
					return nil
				}
				cp := CodePrices{code: code, prices: prices}
				if err := sp.db.InsertDB("daily", cp.Slices()); err != nil {
					return fmt.Errorf("failed to insertCodePricesToDB: %w", err)
				}

				log.Printf("code %s latency: %v", code, time.Since(s))
				return nil
			})
		}
	}

	return failedCodes, eg.Wait()
}

// CodePricesをstringの2重配列にしてDBに格納する関数
func (sp DailyStockPrice) insertCodePricesToDB(csp CodePrices) error {
	var codePrices [][]string

	for _, p := range csp.prices {
		codePrices = append(codePrices, []string{csp.code, p.date, p.open, p.high, p.low, p.close, p.turnover, p.modified})
	}
	//log.Println("stockprice:", codePrices)
	return sp.db.InsertDB("daily", codePrices)
}

func (sp DailyStockPrice) scrape(ctx context.Context, code string) ([]DatePrice, error) {
	if sp.currentTime.IsZero() {
		log.Println("currentTime is zero")
		return nil, fmt.Errorf("currentTime is zero: %#v", sp.currentTime)
	}
	doc, err := sp.fetch(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %v", err)
	}

	var datePrices []DatePrice

	doc.Find(".m-tableType01_table table tbody tr").Each(func(i int, s *goquery.Selection) {
		rawDate := s.Find(".a-taC").Text()
		if rawDate == "" {
			err = errors.New("no date empty")
			return
		}

		var date string
		// 正規表現で前後の空白などを除いた日付を取得
		re := regexp.MustCompile(`[0-9]+/[0-9]+`).Copy()
		// 現在時刻を参考にスクレイピングで取得した日付に年をつけたりゼロ埋めする
		if date, err = formatDate(sp.currentTime, re.FindString(rawDate)); err != nil {
			return
		}
		//log.Println("date:", date, "raw", re.FindString(rawDate), sp.currentTime)

		var prices []string
		// 始値, 高値, 安値, 終値, 売買高, 修正後終値を順に取得
		s.Find(".a-taR").Each(func(j int, s2 *goquery.Selection) {
			var p string
			// 得られた値から1,000区切りの","を取り除く
			if p, err = formatPrice(s2.Text()); err != nil {
				return
			}
			//log.Println("price:", p)
			prices = append(prices, p)
		})
		if len(prices) != 6 {
			// 以下の6要素を取れなかったら何かおかしい
			// 始値, 高値, 安値, 終値, 売買高, 修正後終値
			// リダイレクトされて別のページに飛ばされている可能性もある
			// 失敗した銘柄を返す
			err = fmt.Errorf("%s doesn't have enough elems. prices: %v", code, prices)
			return
		}

		p := DatePrice{
			date:     date,
			open:     prices[0],
			high:     prices[1],
			low:      prices[2],
			close:    prices[3],
			turnover: prices[4],
			modified: prices[5],
		}
		//log.Println("stockprice", p)
		datePrices = append(datePrices, p)
	})
	if datePrices == nil {
		h, err := doc.Html()
		if err != nil {
			return nil, fmt.Errorf("failed to get html: %v", err)
		}
		return nil, fmt.Errorf("failed to scrape stockprice. doc: %s", h)
	}

	//log.Println("err:", err) // TODO
	return datePrices, err
}

// 株価のページを取得して*goquery.Document型で返す関数
func (sp DailyStockPrice) fetch(ctx context.Context, code string) (*goquery.Document, error) {
	// requestのfetchTimeout用に新しくctxを用意
	// 以下の方法
	// https://medium.com/congruence-labs/http-request-fetchTimeouts-in-go-for-beginners-fe6445137c90
	ctx, cancel := context.WithTimeout(ctx, sp.fetchTimeout)
	defer cancel()

	// Request the HTML page.
	req, err := http.NewRequest("GET", sp.dailyStockpriceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to NewRequest: %v", err)
	}
	//クエリパラメータに銘柄コードを付与
	value := req.URL.Query()
	value.Add("scode", code)
	req.URL.RawQuery = value.Encode()

	log.Printf("trying to fetch daily stockprice. code: %s, url: %s", code, req.URL.String())
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to DefaultClient.Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		//io.Copy(ioutil.Discard, resp.Body)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}
		return nil, fmt.Errorf("status error: %s, url: '%s'.\nresponse: %s", resp.Status, req.URL.String(), string(body))
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to load html doc. err: %v", err)
	}
	log.Printf("fetched daily stockprice successfully. code: %s", code)
	return doc, nil
}

// 日付に年を追加する関数。現在の日付を元に前の年のものかどうか判断する
// 1/4 のような日付をゼロ埋めして01/04にする
// 例えば8/12 のような形で来たdateは 2018/08/12 にして返す
func formatDate(now time.Time, date string) (string, error) {
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
