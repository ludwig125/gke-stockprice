package sheet

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
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
	// > Deprecated: please use NewService instead. To provide a custom HTTP client, use option.WithHTTPClient. If you are using google.golang.org/api/googleapis/transport.APIKey, use option.WithAPIKey with NewService instead.
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
	Clear() error
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
		return nil, fmt.Errorf("failed to retrieve data from sheet: %v, sheetID: %s, readRange: %s", err, s.SpreadsheetID, s.ReadRange)
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
	if err := s.Clear(); err != nil {
		return fmt.Errorf("failed to clear sheet: %w", err)
	}
	if err := write(s.Service, s.SpreadsheetID, s.ReadRange, inputs); err != nil {
		return fmt.Errorf("failed to write sheet: %w", err)
	}
	return nil
}

// Clear delete all spreadsheet data.
func (s SpreadSheet) Clear() error {
	resp, err := s.Service.Spreadsheets.Values.Clear(s.SpreadsheetID, s.ReadRange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return fmt.Errorf("unable to clear value. %v", err)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return fmt.Errorf("HTTPstatus error. %v", status)
	}
	// 1000行を超えるセルはClearの際に削除して余計な空白セルを増大させないようにする
	if err := s.deleteRowMoreThan(1000); err != nil {
		return fmt.Errorf("failed to deleteRowMoreThan: %w", err)
	}
	return nil
}

// Spreadsheets.Values.Appendを繰り返すと行が増えていく一方なので、rowThresHold行を超えないように削減するためのメソッド
// あまりにも空のセルが増えると、以下のエラーがでてそれ以上書き込めなくなる
// -> This action would increase the number of cells in the workbook above the limit of 5000000 cells., badRequest
// 参考：https://stackoverflow.com/questions/61590412/deleting-empty-cells-from-google-spreadsheet-programatically-to-avoid-5000000-ce
func (s SpreadSheet) deleteRowMoreThan(rowThresHold int64) error {
	sheetProperties, err := s.getSheetProperties()
	if err != nil {
		return fmt.Errorf("failed to getSheetID: %v", err)
	}
	rowCount := sheetProperties.GridProperties.RowCount
	columnCount := sheetProperties.GridProperties.ColumnCount
	log.Printf("SpreadsheetID: %s, ReadRange: %s, RowCount: %d, ColumnCount: %d", s.SpreadsheetID, s.ReadRange, rowCount, columnCount)
	if rowCount <= rowThresHold {
		// 行数がrowThresHoldを超えていなければ何もしないで終わる
		log.Printf("RowCount: %d is not more than %d, no need to delete rows", rowCount, rowThresHold)
		return nil
	}
	// TODO: rowCountとrowThresHoldのアラート出してもいいかも
	log.Printf("RowCount: %d is over rowThresHold: %d, delete rows", rowCount, rowThresHold)

	req := sheets.Request{
		DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				Dimension:  "ROWS",
				StartIndex: rowThresHold, // spreadsheetの番号でthresHold+1行目以降を削除する。thresHoldが10なら11行目以降を削除する
				EndIndex:   rowCount,     // rowCountまでを削除する
				SheetId:    sheetProperties.SheetId,
			},
		},
	}
	rb := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	resp, err := s.Service.Spreadsheets.BatchUpdate(s.SpreadsheetID, rb).Do()
	if err != nil {
		return fmt.Errorf("unable to BatchUpdate: %v, rowThresHold: %d", err, rowThresHold)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return fmt.Errorf("HTTPstatus error. %v", status)
	}
	return nil
}

// propertiesの中身: https://developers.google.com/sheets/api/samples/sheet#determine_sheet_id_and_other_properties
func (s SpreadSheet) getSheetProperties() (*sheets.SheetProperties, error) {
	spreadSheet, err := s.getSpreadsheet()
	if err != nil {
		return nil, fmt.Errorf("failed to getSpreadsheet: %v", err)
	}

	// spreadsheetのsheetIDなどを取得する
	// spreadsheetidとsheetidは別物なので注意
	// https://stackoverflow.com/questions/46696168/google-sheets-api-addprotectedrange-error-no-grid-with-id-0
	/*
		ref: https://developers.google.com/sheets/api/samples/sheet#determine_sheet_id_and_other_properties
			{
			"sheets": [
				{
				"properties": {
					"sheetId": 867266606,
					"title": "Sheet1",
					"index": 0,
					"sheetType": "GRID",
					"gridProperties": {
					"rowCount": 100,
					"columnCount": 20,
					"frozenRowCount": 1
					}
					"tabColor": {
					"blue": 1.0
					}
				},
				...
			],
			}
	*/
	var sheetTitles []string
	for _, sheet := range spreadSheet.Sheets {
		sheetTitles = append(sheetTitles, sheet.Properties.Title)
		if sheet.Properties.Title == s.ReadRange {
			return sheet.Properties, nil
		}
	}
	return nil, fmt.Errorf("failed to match sheet title. SpreadsheetID: %s, ReadRange: %s, sheet titles: %v", s.SpreadsheetID, s.ReadRange, sheetTitles)
}

func (s SpreadSheet) getSpreadsheet() (*sheets.Spreadsheet, error) {
	// IncludeGridDataをTrueにすると、SpreadsheetのSheetの行数、列数なども取得できる
	// ref: https://developers.google.com/sheets/api/samples/sheet#determine_sheet_id_and_other_properties
	// https://developers.google.com/sheets/api/reference/rest/v4/spreadsheets#sheetproperties
	// https://developers.google.com/sheets/api/reference/rest/v4/spreadsheets/get

	// Rangesメソッドでシートの絞り込みをしないとSpreadsheet内の全シートの情報が得られる
	includeGridData := true
	resp, err := s.Service.Spreadsheets.Get(s.SpreadsheetID).Ranges(s.ReadRange).IncludeGridData(includeGridData).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %v, sheetID: %s, readRange: %s", err, s.SpreadsheetID, s.ReadRange)
	}
	status := resp.ServerResponse.HTTPStatusCode
	if status != 200 {
		return nil, fmt.Errorf("error HTTPstatus: %v", status)
	}
	return resp, nil
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
