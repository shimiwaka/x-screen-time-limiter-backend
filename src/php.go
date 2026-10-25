package main

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Helpers reproducing PHP's loose typing so that requests and data.json are
// interpreted exactly as the former sync.php did.

func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case json.Number:
		f, err := t.Float64()
		return err != nil || f != 0
	case string:
		return t != "" && t != "0"
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	}
	return true
}

func tokenKey(v any) (string, bool) {
	if !truthy(v) {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case json.Number:
		return strconv.FormatInt(toInt(t), 10), true
	case bool:
		return "1", true
	}
	return "", false
}

var numericPrefix = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?`)

func toInt(v any) int64 {
	switch t := v.(type) {
	case bool:
		if t {
			return 1
		}
	case json.Number:
		return numberToInt(string(t))
	case string:
		return numberToInt(strings.TrimLeft(numericPrefix.FindString(t), " \t\n\r\v\f"))
	case []any, map[string]any:
		if count(t) > 0 {
			return 1
		}
	}
	return 0
}

func numberToInt(s string) int64 {
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || f >= math.MaxInt64 || f < math.MinInt64 {
		return 0
	}
	return int64(f)
}

func asMap(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case []any:
		m := make(map[string]any, len(t))
		for i, e := range t {
			m[strconv.Itoa(i)] = e
		}
		return m
	}
	return nil
}

func elem(v any, i int) any {
	switch t := v.(type) {
	case []any:
		if i < len(t) {
			return t[i]
		}
	case map[string]any:
		return t[strconv.Itoa(i)]
	}
	return nil
}

func count(v any) int {
	switch t := v.(type) {
	case []any:
		return len(t)
	case map[string]any:
		return len(t)
	}
	return -1
}
