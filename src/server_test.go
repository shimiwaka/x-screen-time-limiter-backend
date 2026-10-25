package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func hours(pairs ...int64) []int64 {
	h := make([]int64, 24)
	for i := 0; i+1 < len(pairs); i += 2 {
		h[pairs[i]] = pairs[i+1]
	}
	return h
}

func hoursJSON(pairs ...int64) string {
	b, _ := json.Marshal(hours(pairs...))
	return string(b)
}

func serve(t *testing.T, s *server, method, pathInfo, body string) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("PATH_INFO", pathInfo)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(method, "/sync.php"+pathInfo, strings.NewReader(body)))
	return rec
}

func newServer(t *testing.T, data string) *server {
	t.Helper()
	file := filepath.Join(t.TempDir(), "data.json")
	if data != "" {
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return &server{dataFile: file}
}

func readStore(t *testing.T, s *server) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(s.dataFile)
	if err != nil {
		t.Fatal(err)
	}
	var store map[string]json.RawMessage
	if err := json.Unmarshal(b, &store); err != nil {
		t.Fatal(err)
	}
	return store
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("invalid json %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("invalid json %q: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRouting(t *testing.T) {
	tests := []struct {
		method, path, want string
	}{
		{"GET", "", `{"pong":true}`},
		{"GET", "/", `{"pong":true}`},
		{"POST", "/ping", `{"pong":true}`},
		{"OPTIONS", "/sync", ``},
		{"GET", "/sync", `{"error":"not found","path":"/sync"}`},
		{"GET", "/foo", `{"error":"not found","path":"/foo"}`},
	}
	for _, tt := range tests {
		rec := serve(t, newServer(t, ""), tt.method, tt.path, "")
		if rec.Code != http.StatusOK || rec.Body.String() != tt.want {
			t.Errorf("%s %q: got %d %q, want %q", tt.method, tt.path, rec.Code, rec.Body.String(), tt.want)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("%s %q: Access-Control-Allow-Origin = %q", tt.method, tt.path, got)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("%s %q: Content-Type = %q", tt.method, tt.path, got)
		}
	}
}

func TestSyncRequestValidation(t *testing.T) {
	tests := []struct {
		body, want string
	}{
		{``, `{"error":"invalid json"}`},
		{`{`, `{"error":"invalid json"}`},
		{`{} {}`, `{"error":"invalid json"}`},
		{`{}`, `{"error":"invalid json"}`},
		{`[]`, `{"error":"invalid json"}`},
		{`0`, `{"error":"invalid json"}`},
		{`"abc"`, `{"error":"token required"}`},
		{`{"usage":{}}`, `{"error":"token required"}`},
		{`{"token":""}`, `{"error":"token required"}`},
		{`{"token":"0"}`, `{"error":"token required"}`},
		{`{"token":0}`, `{"error":"token required"}`},
		{`{"token":false}`, `{"error":"token required"}`},
	}
	for _, tt := range tests {
		s := newServer(t, "")
		rec := serve(t, s, "POST", "/sync", tt.body)
		if rec.Body.String() != tt.want {
			t.Errorf("body %q: got %q, want %q", tt.body, rec.Body.String(), tt.want)
		}
		if _, err := os.Stat(s.dataFile); !os.IsNotExist(err) {
			t.Errorf("body %q: data file should not be created", tt.body)
		}
	}
}

func TestSyncMerge(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		data      string
		body      string
		want      string
		wantStore string
	}{
		{
			name:      "new token",
			body:      `{"token":"tok","usage":{"2024-01-01":` + hoursJSON(8, 120, 9, 300) + `,"2024-01-02":` + hoursJSON() + `}}`,
			want:      `{"usage":{"2024-01-01":` + hoursJSON(8, 120, 9, 300) + `},"deletedAt":{}}`,
			wantStore: `{"usage":{"2024-01-01":` + hoursJSON(8, 120, 9, 300) + `},"deletedAt":{}}`,
		},
		{
			name: "max per hour when deletedAt equal",
			data: `{"tok":{"usage":{"2024-01-01":` + hoursJSON(1, 100, 2, 50) + `},"deletedAt":[]}}`,
			body: `{"token":"tok","usage":{"2024-01-01":` + hoursJSON(1, 30, 2, 80, 3, 10) + `,"2024-01-03":` + hoursJSON(0, 5) + `}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON(1, 100, 2, 80, 3, 10) + `,"2024-01-03":` + hoursJSON(0, 5) + `},"deletedAt":{}}`,
		},
		{
			name: "newer deletedAt side wins",
			data: `{"tok":{"usage":{"2024-01-01":` + hoursJSON(1, 100, 2, 100) + `},"deletedAt":{"2024-01-01":{"1":1000}}}}`,
			body: `{"token":"tok","usage":{"2024-01-01":` + hoursJSON(1, 500, 2, 5) + `},"deletedAt":{"2024-01-01":{"2":2000}}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON(1, 100, 2, 5) + `},"deletedAt":{"2024-01-01":{"1":1000,"2":2000}}}`,
		},
		{
			name: "deleted date with zero usage is kept",
			body: `{"token":"tok","usage":{},"deletedAt":{"2024-01-01":{"0":1000,"5":2000}}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON() + `},"deletedAt":{"2024-01-01":{"0":1000,"5":2000}}}`,
		},
		{
			name: "php list form deletedAt",
			data: `{"tok":{"usage":{"2024-01-01":` + hoursJSON(0, 10, 1, 10) + `},"deletedAt":{"2024-01-01":[3000,3000]}}}`,
			body: `{"token":"tok","usage":{"2024-01-01":` + hoursJSON(0, 99, 1, 99) + `},"deletedAt":{"2024-01-01":{"0":1000}}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON(0, 10, 1, 10) + `},"deletedAt":{"2024-01-01":{"0":3000,"1":3000}}}`,
		},
		{
			name:      "legacy format without usage key",
			data:      `{"tok":{"2024-01-01":` + hoursJSON(4, 40) + `}}`,
			body:      `{"token":"tok","usage":{}}`,
			want:      `{"usage":{"2024-01-01":` + hoursJSON(4, 40) + `},"deletedAt":{}}`,
			wantStore: `{"usage":{"2024-01-01":` + hoursJSON(4, 40) + `},"deletedAt":{}}`,
		},
		{
			name:      "deletedHours is dropped",
			data:      `{"tok":{"usage":[],"deletedAt":[],"deletedHours":{"2024-01-01":[1]}}}`,
			body:      `{"token":"tok"}`,
			want:      `{"usage":{},"deletedAt":{}}`,
			wantStore: `{"usage":{},"deletedAt":{}}`,
		},
		{
			name: "invalid hour arrays are treated as zero",
			data: `{"tok":{"usage":{"2024-01-01":[1,2,3]},"deletedAt":{}}}`,
			body: `{"token":"tok","usage":{"2024-01-02":"x","2024-01-03":` + hoursJSON(0, 7) + `}}`,
			want: `{"usage":{"2024-01-03":` + hoursJSON(0, 7) + `},"deletedAt":{}}`,
		},
		{
			name: "loose values are cast like php",
			body: `{"token":"tok","usage":{"2024-01-01":["12abc",1.9,true,null,-5,"",0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON(0, 12, 1, 1, 2, 1) + `},"deletedAt":{}}`,
		},
		{
			name: "numeric token",
			key:  "123",
			body: `{"token":123,"usage":{"2024-01-01":` + hoursJSON(0, 1) + `}}`,
			want: `{"usage":{"2024-01-01":` + hoursJSON(0, 1) + `},"deletedAt":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newServer(t, tt.data)
			rec := serve(t, s, "POST", "/sync", tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			assertJSONEqual(t, rec.Body.String(), tt.want)

			key := tt.key
			if key == "" {
				key = "tok"
			}
			entry, ok := readStore(t, s)[key]
			if !ok {
				t.Fatalf("token %q not stored", key)
			}
			if tt.wantStore != "" {
				assertJSONEqual(t, string(entry), tt.wantStore)
			}
		})
	}
}

func TestSyncPreservesOtherTokens(t *testing.T) {
	other := `{"usage":{"2024-01-01":` + hoursJSON(0, 1) + `},"deletedAt":[],"extra":12345678901234567890}`
	s := newServer(t, `{"other":`+other+`}`)
	serve(t, s, "POST", "/sync", `{"token":"tok","usage":{"2024-01-01":`+hoursJSON(0, 2)+`}}`)

	store := readStore(t, s)
	if string(store["other"]) != other {
		t.Errorf("other token changed:\ngot  %s\nwant %s", store["other"], other)
	}
	if _, ok := store["tok"]; !ok {
		t.Error("tok not stored")
	}
}

func TestSyncCorruptDataFile(t *testing.T) {
	s := newServer(t, `{"tok":`)
	rec := serve(t, s, "POST", "/sync", `{"token":"tok"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	b, _ := os.ReadFile(s.dataFile)
	if string(b) != `{"tok":` {
		t.Errorf("corrupt data file was overwritten: %s", b)
	}
}
