// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package netfetch

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = time.Sleep })
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(hits.Add(1)) - 1
		w.Header().Set("Location", "/next")
		w.WriteHeader(statuses[min(n, len(statuses)-1)])
		_, _ = io.WriteString(w, "body")
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func get(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	return Do(http.DefaultClient, req)
}

func TestRetriesServerErrorThenSucceeds(t *testing.T) {
	srv, hits := serve(t, 503, 502, 200)
	resp, err := get(t, srv.URL)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "body", string(body), "the body outlives Do's timeout context")
	assert.EqualValues(t, 3, hits.Load())
}

func TestClientErrorIsNotRetried(t *testing.T) {
	srv, hits := serve(t, 404)
	resp, err := get(t, srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 404, resp.StatusCode)
	assert.EqualValues(t, 1, hits.Load())
}

func TestPersistentServerErrorIsReturned(t *testing.T) {
	srv, hits := serve(t, 500)
	resp, err := get(t, srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 500, resp.StatusCode)
	assert.EqualValues(t, attempts, hits.Load())
}

func TestTimeoutAppliesPerAttempt(t *testing.T) {
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = time.Sleep; SetTimeout(0) })
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	SetTimeout(20 * time.Millisecond)
	start := time.Now()
	_, err := get(t, srv.URL)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestSetTimeoutRestoresDefault(t *testing.T) {
	SetTimeout(time.Second)
	SetTimeout(0)
	assert.Equal(t, DefaultTimeout, Timeout())
}

func TestRefusedRedirectIsNotRetried(t *testing.T) {
	srv, hits := serve(t, 302)
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("refused") }}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = Do(c, req)
	require.ErrorContains(t, err, "refused")
	assert.EqualValues(t, 1, hits.Load())
}

func TestUnreachableHostIsRetried(t *testing.T) {
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = time.Sleep })
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	attemptsSeen := 0
	c := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attemptsSeen++
		return http.DefaultTransport.RoundTrip(r)
	})}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = Do(c, req)
	require.Error(t, err)
	assert.Equal(t, attempts, attemptsSeen)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
