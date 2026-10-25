package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"syscall"
)

type server struct {
	dataFile string
}

type errorResponse struct {
	Error string `json:"error"`
}

type notFoundResponse struct {
	Error string `json:"error"`
	Path  string `json:"path"`
}

type syncResponse struct {
	Usage     map[string][24]int64     `json:"usage"`
	DeletedAt map[string]map[int]int64 `json:"deletedAt"`
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		return
	}

	path := os.Getenv("PATH_INFO")
	switch {
	case path == "" || path == "/" || path == "/ping":
		io.WriteString(w, `{"pong":true}`)
	case path == "/sync" && r.Method == http.MethodPost:
		s.handleSync(w, r)
	default:
		writeJSON(w, http.StatusOK, notFoundResponse{Error: "not found", Path: path})
	}
}

func (s *server) handleSync(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusOK, errorResponse{Error: "invalid json"})
		return
	}
	req, err := decodeJSON(body)
	if err != nil || !truthy(req) {
		writeJSON(w, http.StatusOK, errorResponse{Error: "invalid json"})
		return
	}
	fields, _ := req.(map[string]any)
	token, ok := tokenKey(fields["token"])
	if !ok {
		writeJSON(w, http.StatusOK, errorResponse{Error: "token required"})
		return
	}

	res, err := s.sync(token, asMap(fields["usage"]), asMap(fields["deletedAt"]))
	if err != nil {
		log.Print(err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) sync(token string, clientUsage, clientDeletedAt map[string]any) (*syncResponse, error) {
	f, err := os.OpenFile(s.dataFile, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("failed to lock data file: %w", err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}
	store, err := parseStore(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data file: %w", err)
	}
	entry, err := parseEntry(store[token])
	if err != nil {
		return nil, fmt.Errorf("failed to parse entry: %w", err)
	}

	usage, deletedAt := merge(asMap(entry["usage"]), asMap(entry["deletedAt"]), clientUsage, clientDeletedAt)
	entry["usage"] = usage
	entry["deletedAt"] = deletedAt

	if store[token], err = json.Marshal(entry); err != nil {
		return nil, fmt.Errorf("failed to encode entry: %w", err)
	}
	out, err := json.Marshal(store)
	if err != nil {
		return nil, fmt.Errorf("failed to encode store: %w", err)
	}
	if _, err := f.WriteAt(out, 0); err != nil {
		return nil, fmt.Errorf("failed to write data file: %w", err)
	}
	if err := f.Truncate(int64(len(out))); err != nil {
		return nil, fmt.Errorf("failed to truncate data file: %w", err)
	}
	return &syncResponse{Usage: usage, DeletedAt: deletedAt}, nil
}

func parseStore(data []byte) (map[string]json.RawMessage, error) {
	store := map[string]json.RawMessage{}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return store, nil
	}
	if data[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		for i, v := range list {
			store[strconv.Itoa(i)] = v
		}
		return store, nil
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store == nil {
		store = map[string]json.RawMessage{}
	}
	return store, nil
}

func parseEntry(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	entry, ok := v.(map[string]any)
	if !ok {
		return map[string]any{"usage": v}, nil
	}
	if _, ok := entry["usage"]; !ok {
		return map[string]any{"usage": entry}, nil
	}
	delete(entry, "deletedHours")
	return entry, nil
}

func merge(serverUsage, serverDeletedAt, clientUsage, clientDeletedAt map[string]any) (map[string][24]int64, map[string]map[int]int64) {
	dates := map[string]struct{}{}
	for _, m := range []map[string]any{serverUsage, serverDeletedAt, clientUsage, clientDeletedAt} {
		for date := range m {
			dates[date] = struct{}{}
		}
	}

	usage := map[string][24]int64{}
	deletedAt := map[string]map[int]int64{}
	for date := range dates {
		serverHours := hoursOf(serverUsage[date])
		clientHours := hoursOf(clientUsage[date])

		var hours [24]int64
		deleted := map[int]int64{}
		hasUsage := false
		for h := 0; h < 24; h++ {
			sd := toInt(elem(serverDeletedAt[date], h))
			cd := toInt(elem(clientDeletedAt[date], h))
			if md := maxInt(sd, cd); md > 0 {
				deleted[h] = md
			}

			var val int64
			if sd >= cd {
				val = maxInt(val, serverHours[h])
			}
			if cd >= sd {
				val = maxInt(val, clientHours[h])
			}
			hours[h] = val
			if val > 0 {
				hasUsage = true
			}
		}

		if !hasUsage && len(deleted) == 0 {
			continue
		}
		usage[date] = hours
		if len(deleted) > 0 {
			deletedAt[date] = deleted
		}
	}
	return usage, deletedAt
}

func hoursOf(v any) [24]int64 {
	var hours [24]int64
	if count(v) != 24 {
		return hours
	}
	for h := range hours {
		hours[h] = toInt(elem(v, h))
	}
	return hours
}

func maxInt(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(b)
}

var errTrailingData = errors.New("trailing data after JSON value")

func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errTrailingData
	}
	return v, nil
}
