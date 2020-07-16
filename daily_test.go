package main

import (
	"reflect"
	"testing"

	//_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

func TestFailedCodesSlice(t *testing.T) {
	tests := map[string]struct {
		failedCodes FailedCodes
		wants       []string
	}{
		"no_fcodes": {
			failedCodes: FailedCodes{},
			wants:       []string{},
		},
		"2fcodes": {
			failedCodes: FailedCodes{FailedCode{err: errors.New("90000 error"), code: "90000"}, FailedCode{err: errors.New("90001 error"), code: "90001"}},
			wants:       []string{"90000", "90001"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fcodes := failedCodesSlice(tc.failedCodes)
			if !reflect.DeepEqual(fcodes, tc.wants) {
				t.Fatalf("got: %#v, want: %v", fcodes, tc.wants)
			}
		})
	}
}

func TestFilterSuccessCodes(t *testing.T) {
	tests := map[string]struct {
		codes             []string
		failedCodes       FailedCodes
		wantFilteredCodes []string
	}{
		"no_filter": {
			codes:             []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			failedCodes:       FailedCodes{},
			wantFilteredCodes: []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
		},
		"filter_2codes": {
			codes:             []string{"1802", "2587", "90000", "90001", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			failedCodes:       FailedCodes{FailedCode{err: errors.New("90000 error"), code: "90000"}, FailedCode{err: errors.New("90001 error"), code: "90001"}},
			wantFilteredCodes: []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
		},
		"filter_4codes": {
			codes:             []string{"1802", "2587", "90000", "90001", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
			failedCodes:       FailedCodes{FailedCode{err: errors.New("1802 error"), code: "1802"}, FailedCode{err: errors.New("2587 error"), code: "2587"}, FailedCode{err: errors.New("90000 error"), code: "90000"}, FailedCode{err: errors.New("90001 error"), code: "90001"}},
			wantFilteredCodes: []string{"3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			filteredCodes := filterSuccessCodes(tc.codes, tc.failedCodes)
			if !reflect.DeepEqual(filteredCodes, tc.wantFilteredCodes) {
				t.Fatalf("got filteredCodes: %#v, want wantFilteredCodes: %v", filteredCodes, tc.wantFilteredCodes)
			}
		})
	}
}
