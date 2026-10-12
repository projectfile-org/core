// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package netfetch is the one HTTP policy every core fetch shares: a settable per-request timeout, a settable attempt count and bounded backoff with jitter.
package netfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

// DefaultTimeout bounds one request attempt until SetTimeout changes it.
const DefaultTimeout = 10 * time.Second

// DefaultAttempts is the total tries per request, counting the first, until SetAttempts changes it.
const DefaultAttempts = 3

// baseDelay doubles between attempts; MaxDelay caps the growth.
const (
	baseDelay = 250 * time.Millisecond
	MaxDelay  = 4 * time.Second
)

var (
	timeout  atomic.Int64
	attempts atomic.Int64
)

func init() {
	timeout.Store(int64(DefaultTimeout))
	attempts.Store(int64(DefaultAttempts))
}

// SetTimeout sets the per-attempt timeout; zero or less restores DefaultTimeout.
func SetTimeout(d time.Duration) {
	if d <= 0 {
		d = DefaultTimeout
	}
	timeout.Store(int64(d))
}

// Timeout reports the per-attempt timeout in force.
func Timeout() time.Duration { return time.Duration(timeout.Load()) }

// SetAttempts sets the total tries per request; one or less restores DefaultAttempts.
func SetAttempts(n int) {
	if n < 1 {
		n = DefaultAttempts
	}
	attempts.Store(int64(n))
}

// Attempts reports the total tries per request in force.
func Attempts() int { return int(attempts.Load()) }

// Sleep waits d on the shared schedule and reports whether the caller may try again.
func Sleep(ctx context.Context, d time.Duration) bool { return sleep(ctx, d) }

// Delay reports the wait before attempt (1-based), which doubles per attempt and carries jitter so a fleet of concurrent callers does not retry in lockstep.
func Delay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := baseDelay
	for i := 1; i < attempt && delay < MaxDelay; i++ {
		delay *= 2
	}
	jitter := time.Duration(mrand.Int64N(int64(baseDelay) + 1)) // #nosec G404 -- non-crypto retry jitter
	return delay + jitter
}

// sleep waits d and reports whether the caller may try again; a variable so tests skip the wait.
var sleep = defaultSleep

func defaultSleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// NoBackoff drops the wait between attempts for a sibling package's tests and returns the restore.
func NoBackoff() (restore func()) {
	sleep = func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }
	return func() { sleep = defaultSleep }
}

// Do sends req through c, retrying transport errors and 5xx with backoff and jitter; a replayable body is resent whole on every attempt through req.GetBody, so a retried POST is never sent empty; a body that cannot be replayed is sent once and a retryable failure is then reported as such instead of redelivering a drained stream.
func Do(c *http.Client, req *http.Request) (*http.Response, error) {
	total, replayable := Attempts(), req.Body == nil || req.GetBody != nil
	for attempt := 1; ; attempt++ {
		if attempt > 1 {
			if !replayable {
				return nil, fmt.Errorf("request body cannot be replayed, so attempt %d was the last", attempt-1)
			}
			if !sleep(req.Context(), Delay(attempt)) {
				return nil, fmt.Errorf("http %s %s: %w", req.Method, req.URL.Redacted(), req.Context().Err())
			}
			genlog.Debug("http retry", "url", req.URL.Redacted(), "attempt", attempt, "of", total)
			if err := rewind(req); err != nil {
				return nil, err
			}
		}
		ctx, cancel := context.WithTimeout(req.Context(), Timeout())
		resp, err := c.Do(req.Clone(ctx)) // #nosec G704 -- the caller owns the URL: a user-authored include, the fixed SPDX host, or the forge the user listed
		if err != nil {
			cancel()
			genlog.Debug("http attempt failed", "url", req.URL.Redacted(), "attempt", attempt, "of", total, "err", err.Error())
			if req.Context().Err() != nil || !transient(err) || attempt == total {
				return nil, err
			}
			continue
		}
		if resp.StatusCode/100 == 5 && attempt < total {
			genlog.Debug("http server error", "url", req.URL.Redacted(), "attempt", attempt, "of", total, "status", resp.StatusCode)
			drain(resp.Body)
			cancel()
			continue
		}
		resp.Body = cancelOnClose{resp.Body, cancel}
		return resp, nil
	}
}

// rewind restores req.Body from its GetBody snapshot so the next attempt sends the same bytes as the first.
func rewind(req *http.Request) error {
	if req.Body == nil || req.GetBody == nil {
		return nil
	}
	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("rewind request body: %w", err)
	}
	req.Body = body
	return nil
}

// drain empties a body the retry discards so the underlying connection returns to the pool.
func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<16)) // #nosec G110 -- a discarded body, bounded so it cannot grow
	_ = body.Close()
}

// transient reports a failure worth retrying: a network error, a timeout or a dropped connection, never a client policy refusal.
func transient(err error) bool {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}

// cancelOnClose releases the attempt's timeout context once the caller is done with the body.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
