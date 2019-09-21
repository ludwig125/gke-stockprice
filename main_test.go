package main

import (
	"testing"
)

func TestEnsureDB(t *testing.T) {
	defer SetupTestDB(t)()

	db := NewTestDB(t)
	if err := ensureDB(db); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDBError(t *testing.T) {
	d, err := openSQL("root@/") // DB名を指定せずに接続
	if err != nil {
		t.Error(err)
	}
	db := &MySQL{d}
	wantErr := "database needs to be used. 'select database()': '[[]]'"
	if err := ensureDB(db); err != nil {
		if err.Error() != wantErr {
			t.Fatalf("got error: %v, want error: %v", err, wantErr)
		}
	}
}
