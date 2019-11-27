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
