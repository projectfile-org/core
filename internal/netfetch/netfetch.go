// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package netfetch is the one HTTP policy every core fetch shares: a settable per-request timeout and bounded retry.
package netfetch

import (
	"context"
	"errors"
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

// attempts is the total tries per request; baseDelay doubles between them.
const (
	attempts  = 3
	baseDelay = 250 * time.Millisecond
)

var timeout atomic.Int64

func init() { timeout.Store(int64(DefaultTimeout)) }

// SetTimeout sets the per-attempt timeout; zero or less restores DefaultTimeout.
func SetTimeout(d time.Duration) {
	if d <= 0 {
		d = DefaultTimeout
	}
	timeout.Store(int64(d))
}

// Timeout reports the per-attempt timeout in force.
func Timeout() time.Duration { return time.Duration(timeout.Load()) }

// sleep waits between attempts; a variable so tests skip the wait.
var sleep = time.Sleep

// NoBackoff drops the wait between attempts for a sibling package's tests and returns the restore.
func NoBackoff() (restore func()) {
	sleep = func(time.Duration) {}
	return func() { sleep = time.Sleep }
}

// Do sends a body-less req through c, retrying transport errors and 5xx with backoff and jitter; a 4xx returns at once.
func Do(c *http.Client, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	for attempt := range attempts {
		if attempt > 0 {
			delay := baseDelay<<(attempt-1) + time.Duration(mrand.Int64N(int64(baseDelay))) // #nosec G404 -- non-crypto retry jitter
			genlog.Debug("http retry", "url", req.URL.Redacted(), "attempt", attempt+1, "delay", delay.Round(time.Millisecond))
			sleep(delay)
		}
		ctx, cancel := context.WithTimeout(req.Context(), Timeout())
		resp, err = c.Do(req.Clone(ctx)) // #nosec G704 -- the caller owns the URL: a user-authored include or the fixed SPDX host
		if err != nil {
			cancel()
			genlog.Debug("http attempt failed", "url", req.URL.Redacted(), "attempt", attempt+1, "err", err.Error())
			if req.Context().Err() != nil || !transient(err) {
				return nil, err
			}
			continue
		}
		resp.Body = cancelOnClose{resp.Body, cancel}
		if resp.StatusCode/100 != 5 || attempt == attempts-1 {
			return resp, nil
		}
		genlog.Debug("http server error", "url", req.URL.Redacted(), "attempt", attempt+1, "status", resp.StatusCode)
		_ = resp.Body.Close()
	}
	return resp, err
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
