package server

import (
	"net/http"
	"testing"
)

func TestQueryLoggingKeepsResponses(t *testing.T) {
	app, token, _ := fixture(t)
	h, err := startRouter(t, app, map[string][]string{"deals": {"id", "title"}})
	if err != nil {
		t.Fatal(err)
	}
	if r := request(h, "POST", "/api/collections/deals/records", token, `{"title":"Logged deal"}`); r.Code != 200 {
		t.Fatalf("create: %d %s", r.Code, r.Body)
	}
	ok := request(h, "POST", "/api/context/query", token, `{"sql":"SELECT title FROM deals"}`)
	if ok.Code != http.StatusOK || ok.Header().Get("X-Context-Truncated") != "false" {
		t.Fatalf("query: %d %s", ok.Code, ok.Body)
	}
	bad := request(h, "POST", "/api/context/query", token, `{"sql":"SELECT secret FROM deals","format":"csv"}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("denied query: %d %s", bad.Code, bad.Body)
	}
	if again := request(h, "POST", "/api/context/query", token, `{"sql":"SELECT title FROM deals"}`); again.Code != http.StatusOK {
		t.Fatalf("query after logged failure: %d %s", again.Code, again.Body)
	}
}
