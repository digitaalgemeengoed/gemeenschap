package main

import "testing"

// De sjablonen moeten te lezen zijn (anders start de dienst niet).
func TestSjablonen(t *testing.T) {
	a := &App{}
	a.laadSjablonen()
	if len(a.sjablonen) == 0 {
		t.Fatal("geen sjablonen")
	}
}
