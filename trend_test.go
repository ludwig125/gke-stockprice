// +build !integration

package main

import (
	"testing"
)

func TestCalcContinuationDays(t *testing.T) {
	tests := map[string]struct {
		closes []float64
		want   int
	}{
		"0": {
			closes: []float64{},
			want:   0,
		},
		"0-2": {
			closes: nil,
			want:   0,
		},
		"0-3": {
			closes: []float64{100},
			want:   0,
		},
		"0-4": { // 前と同じ値なら0
			closes: []float64{100, 100},
			want:   0,
		},
		"up-1": {
			closes: []float64{100, 99},
			want:   1,
		},
		"up-1-2": {
			closes: []float64{100, 99, 99},
			want:   1,
		},
		"up-1-3": {
			closes: []float64{100, 99, 100},
			want:   1,
		},
		"up-2": {
			closes: []float64{100, 99, 98},
			want:   2,
		},
		"up-2-2": {
			closes: []float64{100, 99, 98, 98},
			want:   2,
		},
		"up-2-3": {
			closes: []float64{100, 99, 98, 99},
			want:   2,
		},
		"up-3": {
			closes: []float64{100, 99, 98, 97},
			want:   3,
		},
		"up-4": {
			closes: []float64{100, 99, 98, 97, 96},
			want:   4,
		},
		"up-5": {
			closes: []float64{100, 99, 98, 97, 96, 95},
			want:   5,
		},
		"up-6": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94},
			want:   6,
		},
		"up-7": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93},
			want:   7,
		},
		"up-8": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92},
			want:   8,
		},
		"up-9": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91},
			want:   9,
		},
		"up-10": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90},
			want:   10,
		},
		"up-11": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89},
			want:   11,
		},
		"up-12": {
			closes: []float64{100, 99, 98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88},
			want:   11, // 11より上はない maxContinuationDays=11なので
		},
		"down-1": {
			closes: []float64{100, 101},
			want:   1,
		},
		"down-1-2": {
			closes: []float64{100, 101, 101},
			want:   1,
		},
		"down-1-3": {
			closes: []float64{100, 101, 100},
			want:   1,
		},
		"down-2": {
			closes: []float64{100, 101, 102},
			want:   2,
		},
		"down-2-2": {
			closes: []float64{100, 101, 102, 101},
			want:   2,
		},
		"down-2-3": {
			closes: []float64{100, 101, 102, 102},
			want:   2,
		},
		"down-3": {
			closes: []float64{100, 101, 102, 103},
			want:   3,
		},
		"down-4": {
			closes: []float64{100, 101, 102, 103, 104},
			want:   4,
		},
		"down-5": {
			closes: []float64{100, 101, 102, 103, 104, 105},
			want:   5,
		},
		"down-6": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106},
			want:   6,
		},
		"down-7": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107},
			want:   7,
		},
		"down-8": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108},
			want:   8,
		},
		"down-9": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109},
			want:   9,
		},
		"down-10": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110},
			want:   10,
		},
		"down-11": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111},
			want:   11,
		},
		"down-12": {
			closes: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112},
			want:   11,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := calcContinuationDays(tc.closes); got != tc.want {
				t.Errorf("got: %d, want: %d", got, tc.want)
			}
		})
	}
}
