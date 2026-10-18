// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package netfetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// netErr stands in for the net.Error the stdlib returns on a refused connection.
type netErr string

// Error implements error.
func (e netErr) Error() string { return string(e) }

// Timeout implements net.Error.
func (netErr) Timeout() bool { return false }

// Temporary implements net.Error.
func (netErr) Temporary() bool { return true }

// TestDelayDoublesThenCaps pins the backoff schedule at every rung.
func TestDelayDoublesThenCaps(t *testing.T) {
	base := baseDelay

	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{
		{1, base},
		{2, 2 * base},
		{3, 4 * base},
		{4, 8 * base},
	} {
		t.Run(time.Duration(tc.attempt).String(), func(t *testing.T) {
			// Delay adds jitter in [0, base], so the window is [want, want+base].
			for range 20 {
				got := Delay(tc.attempt)
				assert.GreaterOrEqual(t, got, tc.want)
				assert.LessOrEqual(t, got, tc.want+base)
			}
		})
	}
}

// TestDelayClampsAtMaxDelay pins the ceiling.
func TestDelayClampsAtMaxDelay(t *testing.T) {
	for _, attempt := range []int{5, 6, 10, 50, 500} {
		for range 20 {
			got := Delay(attempt)
			assert.GreaterOrEqual(t, got, MaxDelay, "attempt %d", attempt)
			assert.Less(t, got, MaxDelay+baseDelay,
				"attempt %d must not exceed the cap plus jitter", attempt)
		}
	}
}

// TestDelayClampsAttemptBelowOne pins the 1-based contract.
func TestDelayClampsAttemptBelowOne(t *testing.T) {
	for _, attempt := range []int{0, -1, -100} {
		got := Delay(attempt)
		assert.GreaterOrEqual(t, got, baseDelay, "attempt %d", attempt)
		assert.LessOrEqual(t, got, baseDelay+baseDelay, "attempt %d", attempt)
	}
}

// TestDelayCarriesJitter pins that two callers do not get the identical wait.
func TestDelayCarriesJitter(t *testing.T) {
	seen := map[time.Duration]bool{}
	for range 50 {
		seen[Delay(1)] = true
	}
	assert.Greater(t, len(seen), 1, "a fixed wait would re-try a whole fleet in lockstep")
}

// TestSettersClampToDefaults pins the knob contract.
func TestSettersClampToDefaults(t *testing.T) {
	prevTimeout, prevAttempts := Timeout(), Attempts()
	t.Cleanup(func() { SetTimeout(prevTimeout); SetAttempts(prevAttempts) })

	for _, d := range []time.Duration{0, -time.Second} {
		SetTimeout(d)
		assert.Equal(t, DefaultTimeout, Timeout(), "d %v", d)
	}
	SetTimeout(3 * time.Second)
	assert.Equal(t, 3*time.Second, Timeout())

	for _, n := range []int{0, -1, -100} {
		SetAttempts(n)
		assert.Equal(t, DefaultAttempts, Attempts(), "n %d", n)
	}
	SetAttempts(5)
	assert.Equal(t, 5, Attempts())
}

// TestDoRefusesANonReplayableBody pins that a non-rewindable body is sent once.
func TestDoRefusesANonReplayableBody(t *testing.T) {
	restore := NoBackoff()
	t.Cleanup(restore)
	SetAttempts(3)
	t.Cleanup(func() { SetAttempts(DefaultAttempts) })

	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, netErr("dial: connection refused")
	})}

	req, err := http.NewRequest(http.MethodPost, "https://x.test/a", strings.NewReader("payload"))
	require.NoError(t, err)
	req.GetBody = nil

	_, err = Do(client, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be replayed")
	assert.Equal(t, 1, calls, "the first attempt is the only one that can be trusted")
}

// TestDoReplaysABodyWhole pins that a retried POST carries the same bytes.
func TestDoReplaysABodyWhole(t *testing.T) {
	restore := NoBackoff()
	t.Cleanup(restore)
	SetAttempts(3)
	t.Cleanup(func() { SetAttempts(DefaultAttempts) })

	var seen []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		seen = append(seen, string(b))
		if len(seen) < 2 {
			return nil, netErr("dial: connection refused")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     http.Header{},
		}, nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://x.test/a", strings.NewReader("payload"))
	require.NoError(t, err)
	require.NotNil(t, req.GetBody, "NewRequest on a strings.Reader sets GetBody")

	resp, err := Do(client, req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Len(t, seen, 2)
	assert.Equal(t, "payload", seen[0])
	assert.Equal(t, "payload", seen[1], "the retried attempt must not send an empty body")
}

// TestDoCountsOneRequestPerRetryBudget pins the --timing counter.
func TestDoCountsOneRequestPerRetryBudget(t *testing.T) {
	restore := NoBackoff()
	t.Cleanup(restore)
	SetAttempts(3)
	t.Cleanup(func() { SetAttempts(DefaultAttempts) })

	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, netErr("dial: connection refused")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     http.Header{},
		}, nil
	})}

	ResetRequests()
	req, err := http.NewRequest(http.MethodGet, "https://x.test/a", nil)
	require.NoError(t, err)

	resp, err := Do(client, req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, 3, calls, "three transport attempts")
	assert.Equal(t, int64(1), Requests(), "one logical crossing")
}

// TestTransientClassifiesOnlyRetryableFailures pins what is worth retrying.
func TestTransientClassifiesOnlyRetryableFailures(t *testing.T) {
	retryable := []error{
		netErr("dial tcp: connection refused"),
		netErr("i/o timeout"),
		context.DeadlineExceeded,
		io.ErrUnexpectedEOF,
		io.EOF,
	}
	for _, err := range retryable {
		assert.True(t, transient(err), "err %v", err)
	}

	notRetryable := []error{
		errors.New("unsupported protocol scheme"),
		errors.New("malformed URL"),
		nil,
	}
	for _, err := range notRetryable {
		assert.False(t, transient(err), "err %v", err)
	}
}

// TestSleepHonoursContextCancellation pins that a cancelled context refuses a retry.
func TestSleepHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, Sleep(ctx, time.Hour), "a cancelled context must not report a retry as allowed")

	assert.True(t, Sleep(context.Background(), time.Millisecond))
}
