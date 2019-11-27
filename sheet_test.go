package main

import (
	"context"
	"log"
	"testing"

	"google.golang.org/api/sheets/v4"
)

func TestGetSheetData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sheetCredential := mustGetenv("SHEET_CREDENTIAL")
	// spreadsheetのserviceを取得
	srv, err := getSheetClient(ctx, sheetCredential)
	if err != nil {
		t.Fatalf("failed to get sheet service. err: %v", err)
	}
	log.Println("succeeded to get sheet service")

	testSheetID := mustGetenv("TEST_SHEET_ID")

	t.Run("testSheet", func(t *testing.T) {
		testHolidaySheet(t, srv, testSheetID)
		testCodeSheet(t, srv, testSheetID)
	})
}

func testHolidaySheet(t *testing.T, srv *sheets.Service, sid string) {
	si := SpreadSheet{Service: srv,
		SpreadsheetID: sid,
		ReadRange:     "holiday",
	}
	resp, err := si.GetSheetData()
	if err != nil {
		t.Fatalf("failed to GetSheetData: %v", err)
	}
	t.Log(resp[0][0])
	for _, v := range resp {
		t.Log(v[0])
	}
}

func testCodeSheet(t *testing.T, srv *sheets.Service, sid string) {
	si := SpreadSheet{
		Service:       srv,
		SpreadsheetID: sid,
		ReadRange:     "tse-first",
	}
	resp, err := si.GetSheetData()
	if err != nil {
		t.Fatalf("failed to GetSheetData: %v", err)
	}
	t.Log(resp[0][0])
	for _, v := range resp {
		t.Log(v[0])
	}
}
