package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/errors"
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

func TestStrToStrSliceSplitedByComma(t *testing.T) {
	tests := map[string]struct {
		in   string
		want []string
	}{
		"no_split": {
			in:   "abc",
			want: []string{"abc"},
		},
		"no_split_with.": {
			in:   "a.bc",
			want: []string{"a.bc"},
		},
		"splited_by_comma": {
			in:   "a,bc",
			want: []string{"a", "bc"},
		},
		"splited_by_comma2": {
			in:   "a,b,c",
			want: []string{"a", "b", "c"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := strToStrSliceSplitedByComma(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
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
		{"100"},
		{"101"},
		{"102"},
		{"103"},
		{"104"},
		{"105"},
		{"106"},
		{"107"},
	}, nil
}

func (s CodeSpreadSheetMock) Insert([][]string) error {
	return nil
}

func (s CodeSpreadSheetMock) Update([][]string) error {
	return nil
}
func (s CodeSpreadSheetMock) Clear() error {
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

func TestReceivePanic(t *testing.T) {
	g := func() error {
		list := []int{1, 2, 3}
		list[3] = 5
		return nil
	}

	tests := map[string]struct {
		f                 func() error
		wantErrMsg1stLine string
	}{
		"no_error": {
			f: func() error {
				return nil
			},
			wantErrMsg1stLine: "",
		},
		"normanl_error": {
			f: func() error {
				return errors.New("this is error")
			},
			wantErrMsg1stLine: "this is error",
		},
		"panic": {
			f: func() error {
				panic("panic")
			},
			wantErrMsg1stLine: "recovered in function : panic",
		},
		"out_of_range": {
			f: func() error {
				list := []int{1, 2, 3, 4}
				list[4] = 5
				return nil
			},
			wantErrMsg1stLine: "recovered in function : runtime error: index out of range [4] with length 4",
		},
		"call_ohter_func_out_of_range": {
			f: func() error {
				return g()
			},
			wantErrMsg1stLine: "recovered in function : runtime error: index out of range [3] with length 3",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotErr := receivePanic(tc.f)
			t.Logf("error with stack trace: %v", gotErr)

			if gotErr != nil {
				e := strings.Split(gotErr.Error(), "\n")
				if e[0] != tc.wantErrMsg1stLine { // エラーの１行目だけと比べる(Stacktraceの結果は見ない)
					t.Errorf("got: %v\nwant: %v", e[0], tc.wantErrMsg1stLine)
				}
			}
		})
	}
}
