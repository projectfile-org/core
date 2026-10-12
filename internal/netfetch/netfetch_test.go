// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package netfetch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep })
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
	assert.EqualValues(t, Attempts(), hits.Load())
}

func TestTimeoutAppliesPerAttempt(t *testing.T) {
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep; SetTimeout(0) })
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
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep })
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
	assert.Equal(t, Attempts(), attemptsSeen)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSetAttemptsBoundsTotalTries(t *testing.T) {
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep; SetAttempts(0) })

	srv, hits := serve(t, 500)
	SetAttempts(1)
	resp, err := get(t, srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.EqualValues(t, 1, hits.Load(), "one attempt means no retry")

	srv2, hits := serve(t, 500)
	SetAttempts(5)
	_, err = get(t, srv2.URL)
	require.NoError(t, err)
	assert.EqualValues(t, 5, hits.Load())
}

func TestSetAttemptsRestoresDefault(t *testing.T) {
	assert.Equal(t, DefaultAttempts, Attempts())
	SetAttempts(7)
	assert.Equal(t, 7, Attempts())
	SetAttempts(0)
	assert.Equal(t, DefaultAttempts, Attempts(), "zero or less restores the default rather than disabling retries")
}

func TestPostBodyIsReplayedOnRetry(t *testing.T) {
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep })
	const full = "payload"
	var bodies []string
	var mu atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if int(mu.Add(1)) < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, strings.NewReader(full))
	require.NoError(t, err)
	resp, err := Do(http.DefaultClient, req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, []string{full, full, full}, bodies, "each attempt sends the full body")
}

func TestUnreplayableBodyIsNotRetried(t *testing.T) {
	NoBackoff()
	t.Cleanup(func() { sleep = defaultSleep })
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, io.NopCloser(strings.NewReader("stream")))
	require.NoError(t, err)
	_, err = Do(http.DefaultClient, req)
	require.ErrorContains(t, err, "cannot be replayed")
	assert.EqualValues(t, 1, hits.Load(), "a drained stream is sent once, never resent empty")
}

func TestBackoffStopsWhenContextDies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(10*time.Millisecond, cancel)
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listens any more, so every attempt is refused
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = Do(http.DefaultClient, req)
	require.ErrorIs(t, err, context.Canceled, "the backoff wait abandons the retry once the caller is gone")
}

func TestDelayGrowsUnderCap(t *testing.T) {
	for attempt := 1; attempt <= 8; attempt++ {
		base := baseDelay << (attempt - 1)
		if base > MaxDelay {
			base = MaxDelay
		}
		d := Delay(attempt)
		assert.GreaterOrEqual(t, d, base, "attempt %d", attempt)
		assert.LessOrEqual(t, d, base+baseDelay, "attempt %d", attempt)
	}
	assert.LessOrEqual(t, Delay(20), MaxDelay+baseDelay, "the cap bounds the wait")
}

func TestDrainEmptiesADiscardedBody(t *testing.T) {
	body := io.NopCloser(bytes.NewReader([]byte("x")))
	drain(body)
	n, err := body.Read(make([]byte, 1))
	assert.Equal(t, 0, n, "the discarded body is consumed, so the connection returns to the pool")
	assert.ErrorIs(t, err, io.EOF)
}
