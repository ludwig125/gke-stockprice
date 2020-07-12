package main

import (
	"testing"
	"time"

	"github.com/pkg/errors"
)

type HolidaySpreadSheetMock struct {
	ReadSheetRes   [][]string // ReadSheetから返す値
	ReadSheetError error      // ReadSheetから返すエラー
}

func (s HolidaySpreadSheetMock) Read() ([][]string, error) {
	return s.ReadSheetRes, s.ReadSheetError
}

func (s HolidaySpreadSheetMock) Insert([][]string) error {
	return nil
}

func (s HolidaySpreadSheetMock) Update([][]string) error {
	return nil
}

func (s HolidaySpreadSheetMock) Clear() error {
	return nil
}

func TestIsHoliday(t *testing.T) {
	cases := []struct {
		name      string
		sheetRes  [][]string // ReadSheetから返す値
		sheetErr  error      // ReadSheetから返す error
		inputDate time.Time  // ReadSheetから返す値と照合する日
		want      bool
		wantErr   error
	}{
		{
			"holiday",
			[][]string{{"2019/01/01"}, {"2019/01/02"}, {"2019/01/03"}},
			nil,
			time.Date(2019, 1, 3, 0, 0, 0, 0, time.Local),
			true, // sheetから返す値に含まれる日を指定したのでtrue
			nil,
		},
		{
			"notholiday",
			[][]string{{"2019/01/01"}, {"2019/01/02"}, {"2019/01/03"}},
			nil,
			time.Date(2019, 1, 4, 0, 0, 0, 0, time.Local),
			false, // sheetから返す値に含まれない日を指定したのでfalse
			nil,
		},
		{
			"ReadSheet_return_error",
			nil,
			errors.New("failed to fetch data"),
			time.Date(2019, 1, 4, 0, 0, 0, 0, time.Local),
			true,
			errors.New("failed to ReadSheet: failed to fetch data"),
		},
		{
			"ReadSheet_return_nil",
			nil,
			nil,
			time.Date(2019, 1, 4, 0, 0, 0, 0, time.Local),
			true,
			errors.New("no data in holidays"),
		},
		{
			"ReadSheet_return_empty",
			[][]string{},
			nil,
			time.Date(2019, 1, 4, 0, 0, 0, 0, time.Local),
			true,
			errors.New("no data in holidays"),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sheet := HolidaySpreadSheetMock{
				ReadSheetRes:   tt.sheetRes, // Mockが返す値を設定
				ReadSheetError: tt.sheetErr, // Mockが返すエラーを設定
			}
			got, err := isHoliday(sheet, tt.inputDate)
			if err != nil {
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("gotErr: %v, wantErr: %v", err, tt.wantErr)
				}
			}
			if got != tt.want {
				t.Errorf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsSaturdayOrSunday(t *testing.T) {
	cases := []struct {
		name      string
		inputDate time.Time
		want      bool
	}{
		{
			"2019_11_1_is_friday",
			time.Date(2019, 11, 1, 0, 0, 0, 0, time.Local),
			false,
		},
		{
			"2019_11_2_is_saturday",
			time.Date(2019, 11, 2, 0, 0, 0, 0, time.Local),
			true,
		},
		{
			"2019_11_3_is_sunday",
			time.Date(2019, 11, 3, 0, 0, 0, 0, time.Local),
			true,
		},
		{
			"2019_11_4_is_monday",
			time.Date(2019, 11, 4, 0, 0, 0, 0, time.Local),
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := isSaturdayOrSunday(tt.inputDate)
			if got != tt.want {
				t.Errorf("got %t, want %t", got, tt.want)
			}
		})
	}
}
