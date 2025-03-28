package kenall_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/osamingo/go-kenall/v2"
)

var (
	//go:embed testdata/addresses.json
	addressResponse []byte
	//go:embed testdata/cities.json
	cityResponse []byte
	//go:embed testdata/corporation.json
	corporationResponse []byte
	//go:embed testdata/whoami.json
	whoamiResponse []byte
	//go:embed testdata/holidays.json
	holidaysResponse []byte
	//go:embed testdata/search_address.json
	searchAddressResponse []byte
	//go:embed testdata/business_day.json
	businessDaysResponse []byte
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		token      string
		httpClient *http.Client
		endpoint   string
		want       error
	}{
		"Empty token":         {token: "", httpClient: nil, endpoint: "", want: kenall.ErrInvalidArgument},
		"Give token":          {token: "dummy", httpClient: nil, endpoint: "", want: nil},
		"Give token and opts": {token: "dummy", httpClient: &http.Client{}, endpoint: "customize_endpoint", want: nil},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := make([]kenall.ClientOption, 0, 2)
			if c.httpClient != nil {
				opts = append(opts, kenall.WithHTTPClient(c.httpClient))
			}
			if c.endpoint != "" {
				opts = append(opts, kenall.WithEndpoint(c.endpoint))
			}

			cli, err := kenall.NewClient(c.token, opts...)
			if !errors.Is(c.want, err) {
				t.Errorf("give: %v, want: %v", err, c.want)
			}

			if c.httpClient != nil && cli.HTTPClient != c.httpClient {
				t.Errorf("give: %v, want: %v", cli.HTTPClient, c.httpClient)
			}
			if c.endpoint != "" && cli.Endpoint != c.endpoint {
				t.Errorf("give: %v, want: %v", cli.Endpoint, c.endpoint)
			}
		})
	}
}

func TestClient_GetAddress(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		postalCode   string
		checkAsError bool
		wantError    any
		wantJISX0402 string
	}{
		"Normal case":           {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "1008105", checkAsError: false, wantError: nil, wantJISX0402: "13104"},
		"Invalid postal code":   {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "alphabet", checkAsError: false, wantError: kenall.ErrInvalidArgument, wantJISX0402: ""},
		"Not found":             {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "0000000", checkAsError: false, wantError: kenall.ErrNotFound, wantJISX0402: ""},
		"Unauthorized":          {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), postalCode: "0000000", checkAsError: false, wantError: kenall.ErrUnauthorized, wantJISX0402: ""},
		"Payment Required":      {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "4020000", checkAsError: false, wantError: kenall.ErrPaymentRequired, wantJISX0402: ""},
		"Forbidden":             {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "4030000", checkAsError: false, wantError: kenall.ErrForbidden, wantJISX0402: ""},
		"Method Not Allowed":    {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "4050000", checkAsError: false, wantError: kenall.ErrMethodNotAllowed, wantJISX0402: ""},
		"Internal server error": {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "5000000", checkAsError: false, wantError: kenall.ErrInternalServerError, wantJISX0402: ""},
		"Unknown status code":   {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "5030000", checkAsError: true, wantError: fmt.Errorf(""), wantJISX0402: ""},
		"Wrong endpoint":        {endpoint: "", token: "opencollector", ctx: t.Context(), postalCode: "0000000", checkAsError: true, wantError: &url.Error{}, wantJISX0402: ""},
		"Wrong response":        {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), postalCode: "0000001", checkAsError: true, wantError: &json.MarshalerError{}, wantJISX0402: ""},
		"Nil context":           {endpoint: srv.URL, token: "opencollector", ctx: nil, postalCode: "0000000", checkAsError: true, wantError: errors.New("net/http: nil Context"), wantJISX0402: ""},
		"Timeout context":       {endpoint: srv.URL, token: "opencollector", ctx: toctx, postalCode: "1008105", checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantJISX0402: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetAddress(c.ctx, c.postalCode)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.Addresses[0].JISX0402 != c.wantJISX0402 {
				t.Errorf("give: %v, want: %v", res.Addresses[0].JISX0402, c.wantJISX0402)
			}
		})
	}
}

func TestClient_GetCity(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint       string
		token          string
		ctx            context.Context
		prefectureCode string
		checkAsError   bool
		wantError      any
		wantJISX0402   string
	}{
		"Normal case":             {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "13", checkAsError: false, wantError: nil, wantJISX0402: "13101"},
		"Invalid prefecture code": {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "alphabet", checkAsError: false, wantError: kenall.ErrInvalidArgument, wantJISX0402: ""},
		"Not found":               {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "48", checkAsError: false, wantError: kenall.ErrNotFound, wantJISX0402: ""},
		"Unauthorized":            {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), prefectureCode: "00", checkAsError: false, wantError: kenall.ErrUnauthorized, wantJISX0402: ""},
		"Payment Required":        {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "90", checkAsError: false, wantError: kenall.ErrPaymentRequired, wantJISX0402: ""},
		"Forbidden":               {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "91", checkAsError: false, wantError: kenall.ErrForbidden, wantJISX0402: ""},
		"Method Not Allowed":      {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "96", checkAsError: false, wantError: kenall.ErrMethodNotAllowed, wantJISX0402: ""},
		"Internal server error":   {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "92", checkAsError: false, wantError: kenall.ErrInternalServerError, wantJISX0402: ""},
		"Unknown status code":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "94", checkAsError: true, wantError: fmt.Errorf(""), wantJISX0402: ""},
		"Wrong endpoint":          {endpoint: "", token: "opencollector", ctx: t.Context(), prefectureCode: "00", checkAsError: true, wantError: &url.Error{}, wantJISX0402: ""},
		"Wrong response":          {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), prefectureCode: "95", checkAsError: true, wantError: &json.MarshalerError{}, wantJISX0402: ""},
		"Nil context":             {endpoint: srv.URL, token: "opencollector", ctx: nil, prefectureCode: "00", checkAsError: true, wantError: errors.New("net/http: nil Context"), wantJISX0402: ""},
		"Timeout context":         {endpoint: srv.URL, token: "opencollector", ctx: toctx, prefectureCode: "13", checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantJISX0402: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetCity(c.ctx, c.prefectureCode)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.Cities[0].JISX0402 != c.wantJISX0402 {
				t.Errorf("give: %v, want: %v", res.Cities[0].JISX0402, c.wantJISX0402)
			}
		})
	}
}

func TestClient_GetCorporation(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint        string
		token           string
		ctx             context.Context
		corporateNumber string
		checkAsError    bool
		wantError       any
		wantJISX0402    string
	}{
		"Normal case":              {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "2021001052596", checkAsError: false, wantError: nil, wantJISX0402: "13101"},
		"Invalid corporate number": {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "alphabet", checkAsError: false, wantError: kenall.ErrInvalidArgument, wantJISX0402: ""},
		"Not found":                {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000001", checkAsError: false, wantError: kenall.ErrNotFound, wantJISX0402: ""},
		"Unauthorized":             {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), corporateNumber: "2021001052596", checkAsError: false, wantError: kenall.ErrUnauthorized, wantJISX0402: ""},
		"Payment Required":         {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000402", checkAsError: false, wantError: kenall.ErrPaymentRequired, wantJISX0402: ""},
		"Forbidden":                {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000403", checkAsError: false, wantError: kenall.ErrForbidden, wantJISX0402: ""},
		"Method Not Allowed":       {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000405", checkAsError: false, wantError: kenall.ErrMethodNotAllowed, wantJISX0402: ""},
		"Internal server error":    {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000500", checkAsError: false, wantError: kenall.ErrInternalServerError, wantJISX0402: ""},
		"Unknown status code":      {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000503", checkAsError: true, wantError: fmt.Errorf(""), wantJISX0402: ""},
		"Wrong endpoint":           {endpoint: "", token: "opencollector", ctx: t.Context(), corporateNumber: "2021001052596", checkAsError: true, wantError: &url.Error{}, wantJISX0402: ""},
		"Wrong response":           {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), corporateNumber: "0000000000000", checkAsError: true, wantError: &json.MarshalerError{}, wantJISX0402: ""},
		"Nil context":              {endpoint: srv.URL, token: "opencollector", ctx: nil, corporateNumber: "2021001052596", checkAsError: true, wantError: errors.New("net/http: nil Context"), wantJISX0402: ""},
		"Timeout context":          {endpoint: srv.URL, token: "opencollector", ctx: toctx, corporateNumber: "2021001052596", checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantJISX0402: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetCorporation(c.ctx, c.corporateNumber)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.Corporation.JISX0402 != c.wantJISX0402 {
				t.Errorf("give: %v, want: %v", res.Corporation.JISX0402, c.wantJISX0402)
			}
		})
	}
}

func TestClient_GetWhoami(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		checkAsError bool
		wantError    any
		wantAddr     string
	}{
		"Normal case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), checkAsError: false, wantError: nil, wantAddr: "192.168.0.1"},
		"Unauthorized":    {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), checkAsError: false, wantError: kenall.ErrUnauthorized, wantAddr: ""},
		"Wrong endpoint":  {endpoint: "", token: "opencollector", ctx: t.Context(), checkAsError: true, wantError: &url.Error{}, wantAddr: ""},
		"Nil context":     {endpoint: srv.URL, token: "opencollector", ctx: nil, checkAsError: true, wantError: errors.New("net/http: nil Context"), wantAddr: ""},
		"Timeout context": {endpoint: srv.URL, token: "opencollector", ctx: toctx, checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantAddr: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetWhoami(c.ctx)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.RemoteAddress.String() != c.wantAddr {
				t.Errorf("give: %v, want: %v", res.RemoteAddress.String(), c.wantAddr)
			}
		})
	}
}

func TestClient_GetHolidays(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		checkAsError bool
		wantError    any
		wantTitle    string
	}{
		"Normal case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), checkAsError: false, wantError: nil, wantTitle: "元日"},
		"Unauthorized":    {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), checkAsError: false, wantError: kenall.ErrUnauthorized, wantTitle: ""},
		"Wrong endpoint":  {endpoint: "", token: "opencollector", ctx: t.Context(), checkAsError: true, wantError: &url.Error{}, wantTitle: ""},
		"Nil context":     {endpoint: srv.URL, token: "opencollector", ctx: nil, checkAsError: true, wantError: errors.New("net/http: nil Context"), wantTitle: ""},
		"Timeout context": {endpoint: srv.URL, token: "opencollector", ctx: toctx, checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantTitle: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetHolidays(c.ctx)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.Holidays[0].Title != c.wantTitle {
				t.Errorf("give: %v, want: %v", res.Holidays[0].Title, c.wantTitle)
			}
		})
	}
}

func TestClient_GetHolidaysByYear(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		giveYear     int
		checkAsError bool
		wantError    any
		wantLen      int
	}{
		"Normal case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveYear: 2022, checkAsError: false, wantError: nil, wantLen: 16},
		"Empty case":      {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveYear: 1969, checkAsError: false, wantError: nil, wantLen: 0},
		"Unauthorized":    {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), giveYear: 2022, checkAsError: false, wantError: kenall.ErrUnauthorized, wantLen: 0},
		"Wrong endpoint":  {endpoint: "", token: "opencollector", ctx: t.Context(), giveYear: 2022, checkAsError: true, wantError: &url.Error{}, wantLen: 0},
		"Nil context":     {endpoint: srv.URL, token: "opencollector", ctx: nil, giveYear: 2022, checkAsError: true, wantError: errors.New("net/http: nil Context"), wantLen: 0},
		"Timeout context": {endpoint: srv.URL, token: "opencollector", ctx: toctx, giveYear: 2022, checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantLen: 0},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetHolidaysByYear(c.ctx, c.giveYear)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && len(res.Holidays) != c.wantLen {
				t.Errorf("give: %v, want: %v", len(res.Holidays), c.wantLen)
			}
		})
	}
}

func TestClient_GetHolidaysByPeriod(t *testing.T) {
	t.Parallel()

	toctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	srv := runTestingServer(t)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	from, err := time.Parse(kenall.RFC3339DateFormat, "2022-01-01")
	if err != nil {
		t.Fatal(err)
	}

	to, err := time.Parse(kenall.RFC3339DateFormat, "2022-12-31")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		giveFrom     time.Time
		giveTo       time.Time
		checkAsError bool
		wantError    any
		wantLen      int
	}{
		"Normal case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveFrom: from, giveTo: to, checkAsError: false, wantError: nil, wantLen: 16},
		"Empty case":      {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveFrom: from.Add(24 * time.Hour), giveTo: to, checkAsError: false, wantError: nil, wantLen: 0},
		"Unauthorized":    {endpoint: srv.URL, token: "bad_token", ctx: t.Context(), giveFrom: from, giveTo: to, checkAsError: false, wantError: kenall.ErrUnauthorized, wantLen: 0},
		"Wrong endpoint":  {endpoint: "", token: "opencollector", ctx: t.Context(), giveFrom: from, giveTo: to, checkAsError: true, wantError: &url.Error{}, wantLen: 0},
		"Nil context":     {endpoint: srv.URL, token: "opencollector", ctx: nil, giveFrom: from, giveTo: to, checkAsError: true, wantError: errors.New("net/http: nil Context"), wantLen: 0},
		"Timeout context": {endpoint: srv.URL, token: "opencollector", ctx: toctx, giveFrom: from, giveTo: to, checkAsError: true, wantError: kenall.ErrTimeout(context.DeadlineExceeded), wantLen: 0},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetHolidaysByPeriod(c.ctx, c.giveFrom, c.giveTo)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && len(res.Holidays) != c.wantLen {
				t.Errorf("give: %v, want: %v", len(res.Holidays), c.wantLen)
			}
		})
	}
}

func TestClient_GetNormalizeAddress(t *testing.T) {
	t.Parallel()

	srv := runTestingServer(t)
	t.Cleanup(func() {
		srv.Close()
	})

	cases := map[string]struct {
		endpoint        string
		token           string
		ctx             context.Context
		giveAddress     string
		checkAsError    bool
		wantError       any
		wantBlockLotNum string
	}{
		"Normal case":    {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveAddress: "東京都港区六本木六丁目10番1号六本木ヒルズ森タワー18F", checkAsError: false, wantError: nil, wantBlockLotNum: "6-10-1"},
		"Empty case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveAddress: "", checkAsError: true, wantError: kenall.ErrInvalidArgument, wantBlockLotNum: ""},
		"Wrong response": {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveAddress: "wrong", checkAsError: true, wantError: &json.MarshalerError{}, wantBlockLotNum: ""},
		"nil context":    {endpoint: srv.URL, token: "opencollector", ctx: nil, giveAddress: "東京都港区六本木六丁目10番1号六本木ヒルズ森タワー18F", checkAsError: true, wantError: &url.Error{}, wantBlockLotNum: ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetNormalizeAddress(c.ctx, c.giveAddress)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.Query.BlockLotNum.String != c.wantBlockLotNum {
				t.Errorf("give: %v, want: %v", res.Query.BlockLotNum, c.wantBlockLotNum)
			}
		})
	}
}

func TestClient_GetBusinessDays(t *testing.T) {
	t.Parallel()

	srv := runTestingServer(t)
	t.Cleanup(func() {
		srv.Close()
	})

	//nolint: maligned
	cases := map[string]struct {
		endpoint     string
		token        string
		ctx          context.Context
		giveTime     time.Time
		checkAsError bool
		wantError    any
		wantResult   bool
	}{
		"Normal case":    {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveTime: time.UnixMilli(1672531200000), checkAsError: false, wantError: nil, wantResult: true},
		"Empty case":     {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveTime: time.Time{}, checkAsError: true, wantError: kenall.ErrInvalidArgument, wantResult: false},
		"Wrong response": {endpoint: srv.URL, token: "opencollector", ctx: t.Context(), giveTime: time.Time{}.Add(24 * time.Hour), checkAsError: true, wantError: &json.MarshalerError{}, wantResult: false},
		"nil context":    {endpoint: srv.URL, token: "opencollector", ctx: nil, giveTime: time.Now(), checkAsError: true, wantError: &url.Error{}, wantResult: false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cli, err := kenall.NewClient(c.token, kenall.WithEndpoint(c.endpoint))
			if err != nil {
				t.Error(err)
			}

			res, err := cli.GetBusinessDays(c.ctx, c.giveTime)
			if c.checkAsError && !errors.As(err, &c.wantError) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			} else if want, ok := c.wantError.(error); ok && !errors.Is(err, want) {
				t.Errorf("give: %v, want: %v", err, c.wantError)
			}
			if res != nil && res.BusinessDay.LegalHoliday != c.wantResult {
				t.Errorf("give: %v, want: %v", res.BusinessDay.LegalHoliday, c.wantResult)
			}
		})
	}
}

func ExampleClient_GetAddress() {
	if testing.Short() {
		// stab
		fmt.Print("false\n東京都 千代田区 千代田\n")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetAddress(context.Background(), "1000001")
	if err != nil {
		log.Fatal(err)
	}

	addr := res.Addresses[0]
	fmt.Println(time.Time(res.Version).IsZero())
	fmt.Println(addr.Prefecture, addr.City, addr.Town)
	// Output:
	// false
	// 東京都 千代田区 千代田
}

func ExampleClient_GetCity() {
	if testing.Short() {
		// stab
		fmt.Print("false\n東京都 千代田区\n")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetCity(context.Background(), "13")
	if err != nil {
		log.Fatal(err)
	}

	addr := res.Cities[0]
	fmt.Println(time.Time(res.Version).IsZero())
	fmt.Println(addr.Prefecture, addr.City)
	// Output:
	// false
	// 東京都 千代田区
}

func ExampleClient_GetCorporation() {
	if testing.Short() {
		// stab
		fmt.Print("false\n東京都 千代田区\n")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetCorporation(context.Background(), "7000012050002")
	if err != nil {
		log.Fatal(err)
	}

	corp := res.Corporation
	fmt.Println(time.Time(res.Version).IsZero())
	fmt.Println(corp.PrefectureName, corp.CityName)
	// Output:
	// false
	// 東京都 千代田区
}

func ExampleClient_GetWhoami() {
	if testing.Short() {
		// stab
		fmt.Println("ip")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetWhoami(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	raddr := res.RemoteAddress
	fmt.Println(raddr.IPAddr.Network())
	// Output:
	// ip
}

func ExampleClient_GetHolidaysByYear() {
	if testing.Short() {
		// stab
		fmt.Println("2022-01-01 元日")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetHolidaysByYear(context.Background(), 2022)
	if err != nil {
		log.Fatal(err)
	}

	day := res.Holidays[0]
	fmt.Println(day.Format(kenall.RFC3339DateFormat), day.Title)
	// Output:
	// 2022-01-01 元日
}

func ExampleClient_GetNormalizeAddress() {
	if testing.Short() {
		// stab
		fmt.Print("false\n3-12-14 8F\n")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetNormalizeAddress(context.Background(), "東京都千代田区麹町三丁目12-14麹町駅前ヒルトップ8F")
	if err != nil {
		log.Fatal(err)
	}

	q := res.Query
	fmt.Println(time.Time(res.Version).IsZero())
	fmt.Println(q.BlockLotNum.String, q.FloorRoom.String)
	// Output:
	// false
	// 3-12-14 8F
}

func ExampleClient_GetBusinessDays() {
	if testing.Short() {
		// stab
		fmt.Print("false\n2000-01-01\n")

		return
	}

	// NOTE: Please set a valid token in the environment variable and run it.
	cli, err := kenall.NewClient(os.Getenv("KENALL_AUTHORIZATION_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	res, err := cli.GetBusinessDays(context.Background(), time.UnixMilli(946684800000)) // 2000-01-01
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res.BusinessDay.LegalHoliday)
	fmt.Println(res.BusinessDay.Format(kenall.RFC3339DateFormat))
	// Output:
	// false
	// 2000-01-01
}

func runTestingServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.Fields(r.Header.Get("Authorization"))

		if len(token) != 2 || token[1] != "opencollector" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		switch uri := r.URL.RequestURI(); {
		case strings.HasPrefix(uri, "/postalcode/"):
			handlePostalAPI(t, w, uri)
		case strings.HasPrefix(uri, "/cities/"):
			handleCityAPI(t, w, uri)
		case strings.HasPrefix(uri, "/houjinbangou/"):
			handleCorporationAPI(t, w, uri)
		case strings.HasPrefix(uri, "/whoami"):
			handleWhoamiAPI(t, w, uri)
		case strings.HasPrefix(uri, "/holidays"):
			handleHolidaysAPI(t, w, uri)
		case strings.HasPrefix(uri, "/businessdays"):
			handleBusinessDaysAPI(t, w, uri)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func handlePostalAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	if strings.HasPrefix(uri, "/postalcode/?") {
		//nolint: errcheck
		u, _ := url.Parse(uri)

		switch u.Query().Get("t") {
		case "東京都港区六本木六丁目10番1号六本木ヒルズ森タワー18F":
			if _, err := w.Write(searchAddressResponse); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		case "wrong":
			if _, err := w.Write([]byte("wrong")); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}

	switch uri {
	case "/postalcode/1008105":
		if _, err := w.Write(addressResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/postalcode/4020000":
		w.WriteHeader(http.StatusPaymentRequired)
	case "/postalcode/4030000":
		w.WriteHeader(http.StatusForbidden)
	case "/postalcode/4050000":
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "/postalcode/5000000":
		w.WriteHeader(http.StatusInternalServerError)
	case "/postalcode/5030000":
		w.WriteHeader(http.StatusServiceUnavailable)
	case "/postalcode/0000001":
		if _, err := w.Write([]byte("wrong")); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/postalcode/0000000":
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleCityAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	switch uri {
	case "/cities/13":
		if _, err := w.Write(cityResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/cities/90":
		w.WriteHeader(http.StatusPaymentRequired)
	case "/cities/91":
		w.WriteHeader(http.StatusForbidden)
	case "/cities/92":
		w.WriteHeader(http.StatusInternalServerError)
	case "/cities/94":
		w.WriteHeader(http.StatusServiceUnavailable)
	case "/cities/95":
		if _, err := w.Write([]byte("wrong")); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/cities/96":
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleCorporationAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	switch uri {
	case "/houjinbangou/2021001052596":
		if _, err := w.Write(corporationResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/houjinbangou/0000000000402":
		w.WriteHeader(http.StatusPaymentRequired)
	case "/houjinbangou/0000000000403":
		w.WriteHeader(http.StatusForbidden)
	case "/houjinbangou/0000000000405":
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "/houjinbangou/0000000000500":
		w.WriteHeader(http.StatusInternalServerError)
	case "/houjinbangou/0000000000503":
		w.WriteHeader(http.StatusServiceUnavailable)
	case "/houjinbangou/0000000000000":
		if _, err := w.Write([]byte("wrong")); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleWhoamiAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	switch uri {
	case "/whoami":
		if _, err := w.Write(whoamiResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleHolidaysAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	switch uri {
	case "/holidays?":
		if _, err := w.Write(holidaysResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	case "/holidays?year=2022":
		if _, err := w.Write(holidaysResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	case "/holidays?from=2022-01-01&to=2022-12-31":
		if _, err := w.Write(holidaysResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	if strings.HasPrefix(uri, "/holidays") {
		if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func handleBusinessDaysAPI(t *testing.T, w http.ResponseWriter, uri string) {
	t.Helper()

	switch uri {
	case "/businessdays/check?date=2023-01-01":
		if _, err := w.Write(businessDaysResponse); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/businessdays/check?date=0001-01-02":
		if _, err := w.Write([]byte(`{"result": "worng"}`)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
