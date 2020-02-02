package sheet

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2/google" // to get sheet client
	"google.golang.org/api/sheets/v4"
)

// GetSheetClient get spread sheet client
// 基本的にこちらに従う
// ref. https://developers.google.com/sheets/api/quickstart/go
// 他参考: https://developers.google.com/sheets/api/quickstart/go#step_3_set_up_the_sample
// spreadsheets clientを取得
func GetSheetClient(ctx context.Context, sheetCredential string) (*sheets.Service, error) {
	// googleAPIへのclientを作成
	client, err := getClientWithJSON(ctx, sheetCredential)
	if err != nil {
		return nil, fmt.Errorf("failed to getClientWithJSON: %v", err)
	}
	// spreadsheets clientを取得
	// TODO: deprecatedなので直す
	// https://godoc.org/google.golang.org/api/sheets/v4#New
	srv, err := sheets.New(client)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Sheets Client: %v", err)
	}
	return srv, nil
}

func getClientWithJSON(ctx context.Context, sheetCredential string) (*http.Client, error) {
	data, err := ioutil.ReadFile(sheetCredential)
	if err != nil {
		return nil, fmt.Errorf("failed to read client secret file. path: '%s', %v", sheetCredential, err)
	}
	// ref. https://godoc.org/golang.org/x/oauth2/google#JWTConfigFromJSON
	// JWTConfigFromJSON uses a Google Developers service account JSON key file to read the credentials that authorize and authenticate the requests.
	conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("failed to parse client secret file to config: %v", err)
	}
	return conf.Client(ctx), nil
}

// Sheet is interface
type Sheet interface {
	Read() ([][]string, error)
	Insert([][]string) error
	Update([][]string) error
}

// SpreadSheet has SpreadsheetID and ReadRange to identify sheet
type SpreadSheet struct {
	Service       *sheets.Service
	SpreadsheetID string // sheetのID
	ReadRange     string // sheetのタブ名
}

// NewSpreadSheet return SpreadSheet
func NewSpreadSheet(srv *sheets.Service, id string, name string) Sheet {
	return SpreadSheet{
		Service:       srv,
		SpreadsheetID: id,
		ReadRange:     name,
	}
}

// ReadSheet fetch data from spread sheet
func (s SpreadSheet) Read() ([][]string, error) {
	// ref. https://developers.google.com/sheets/api/reference/rest/v4/spreadsheets.values/get
	resp, err := s.Service.Spreadsheets.Values.Get(s.SpreadsheetID, s.ReadRange).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve data from sheet: %v", err)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return nil, fmt.Errorf("error HTTPstatus: %v", status)
	}

	// [][]interface{}を[][]stringに変換する
	var res [][]string
	for _, v := range resp.Values {
		var r []string
		for _, v2 := range v {
			str, ok := v2.(string)
			if !ok {
				return nil, fmt.Errorf("failed to convert to string: %v", ok)
			}
			r = append(r, str)
		}
		res = append(res, r)
	}
	return res, nil
}

// Insert write data to spreadsheet without clearing
func (s SpreadSheet) Insert(inputs [][]string) error {
	if err := write(s.Service, s.SpreadsheetID, s.ReadRange, inputs); err != nil {
		return fmt.Errorf("failed to write sheet: %w", err)
	}
	return nil
}

// Update clear spreadsheet and write data
func (s SpreadSheet) Update(inputs [][]string) error {
	if err := clear(s.Service, s.SpreadsheetID, s.ReadRange); err != nil {
		return fmt.Errorf("failed to clear sheet: %w", err)
	}
	if err := write(s.Service, s.SpreadsheetID, s.ReadRange, inputs); err != nil {
		return fmt.Errorf("failed to write sheet: %w", err)
	}
	return nil
}

// sheetService, sheetID, sheet名, 入力データを引数に取り、SpreadSheetに記入する
func write(srv *sheets.Service, sid, srange string, inputs [][]string) error {
	valueRange := &sheets.ValueRange{
		MajorDimension: "ROWS",
		Values:         interfaceSlices(inputs),
	}
	// Write spreadsheet
	resp, err := srv.Spreadsheets.Values.Append(sid, srange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
	if err != nil {
		return fmt.Errorf("unable to write value: %v", err)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return fmt.Errorf("HTTPstatus error: %v", status)
	}
	return nil
}

// interfaceSlices convert two-dimensional string slice to two-dimensional interface slice
func interfaceSlices(sss [][]string) [][]interface{} {
	iss := make([][]interface{}, len(sss))
	for i, ss := range sss {
		is := make([]interface{}, len(ss))
		for j, s := range ss {
			is[j] = s
		}
		iss[i] = is
	}
	return iss
}

func clear(srv *sheets.Service, sid, srange string) error {
	resp, err := srv.Spreadsheets.Values.Clear(sid, srange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return fmt.Errorf("unable to clear value. %v", err)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return fmt.Errorf("HTTPstatus error. %v", status)
	}
	return nil
}

// func Read(r *http.Request, srv *sheets.Service, sheetID string, readRange string) [][]interface{} {
// 	ctx := appengine.NewContext(r)

// 	var MaxRetries = 3
// 	attempt := 0
// 	for {
// 		// MaxRetries を超えていたらnilを返す
// 		if attempt >= MaxRetries {
// 			log.Errorf(ctx, "Failed to retrieve data from sheet. attempt: %d. reached MaxRetries!", attempt)
// 			return nil
// 		}
// 		attempt = attempt + 1
// 		// stockpriceシートからデータを取得
// 		resp, err := srv.Spreadsheets.Values.Get(sheetID, readRange).Do()
// 		if err != nil {
// 			log.Warningf(ctx, "Unable to retrieve data from sheet: %v. attempt: %d", err, attempt)
// 			time.Sleep(1 * time.Second) // 1秒待つ
// 			continue
// 		}
// 		status := resp.ServerResponse.HTTPStatusCode
// 		if status != 200 {
// 			log.Warningf(ctx, "HTTPstatus error: %v. attempt: %d", status, attempt)
// 			time.Sleep(1 * time.Second) // 1秒待つ
// 			continue
// 		}
// 		return resp.Values
// 	}
// }

// func clearSheet(srv *sheets.Service, sid string, sname string) error {
// 	// clear stockprice rate spreadsheet:
// 	resp, err := srv.Spreadsheets.Values.Clear(sid, sname, &sheets.ClearValuesRequest{}).Do()
// 	if err != nil {
// 		return fmt.Errorf("Unable to clear value. %v", err)
// 	}
// 	status := resp.ServerResponse.HTTPStatusCode
// 	if status != 200 {
// 		return fmt.Errorf("HTTPstatus error. %v", status)
// 	}
// 	return nil
// }
