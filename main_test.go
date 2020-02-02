package main

import (
	"reflect"
	"testing"

	sheets "google.golang.org/api/sheets/v4"
)

func TestStrToInt(t *testing.T) {
	tests := map[string]struct {
		in         string
		want       int
		wantErrMsg string
	}{
		"conv_successfully": {
			in:   "1234",
			want: 1234,
		},
		"not_number": {
			in:         "1 2 3 4",
			wantErrMsg: "failed to convert 1 2 3 4 to int",
		},
		"not_int": {
			in:         "1.234",
			wantErrMsg: "failed to convert 1.234 to int",
		},
		"alphabet": {
			in:         "abcd",
			wantErrMsg: "failed to convert abcd to int",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				err := recover()
				if err == nil {
					return
				}
				if err != tc.wantErrMsg {
					t.Errorf("got: %v\nwant: %v", err, tc.wantErrMsg)
				}
			}()

			got := strToInt(tc.in)
			if got != tc.want {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}
}

type CodeSpreadSheetMock struct {
	Service       *sheets.Service
	SpreadsheetID string // sheetのID
	ReadRange     string // sheetのタブ名
}

func (s CodeSpreadSheetMock) Read() ([][]string, error) {
	return [][]string{
		[]string{"100"},
		[]string{"101"},
		[]string{"102"},
		[]string{"103"},
		[]string{"104"},
		[]string{"105"},
		[]string{"106"},
		[]string{"107"},
	}, nil
}

func (s CodeSpreadSheetMock) Insert([][]string) error {
	return nil
}

func (s CodeSpreadSheetMock) Update([][]string) error {
	return nil
}

func TestFetchCompanyCode(t *testing.T) {
	var srv *sheets.Service
	codeSheet := CodeSpreadSheetMock{
		Service:       srv,
		SpreadsheetID: "aaa",
		ReadRange:     "bbb",
	}
	codes, err := fetchCompanyCode(codeSheet)
	if err != nil {
		t.Errorf("failed to fetchCompanyCode: %v", err)
	}
	want := []string{"100", "101", "102", "103", "104", "105", "106", "107"}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("got %v, want %v", codes, want)
	}
}
