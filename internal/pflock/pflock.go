// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pflock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

const (
	lockSuffix    = ".lock"
	defaultWait   = 5 * time.Second
	retryInterval = 100 * time.Millisecond
)

func lockPath(pfPath string) string {
	return pfPath + lockSuffix
}

func WithLock(pfPath string, fn func() error) error {
	return WithLockTimeout(pfPath, defaultWait, fn)
}

func WithLockTimeout(pfPath string, timeout time.Duration, fn func() error) error {
	lp := lockPath(pfPath)
	dir := filepath.Dir(lp)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	fl := flock.New(lp)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	genlog.Debug("acquiring projectfile lock", "path", filepath.Base(lp))

	ok, err := fl.TryLockContext(ctx, retryInterval)
	if err != nil {
		return fmt.Errorf("acquire lock on %s: %w", filepath.Base(pfPath), err)
	}
	if !ok {
		return fmt.Errorf("projectfile %s is locked by another process (timeout after %s)", filepath.Base(pfPath), timeout)
	}
	defer func() {
		if err := fl.Unlock(); err != nil {
			genlog.Warn("release projectfile lock", "path", filepath.Base(lp), "err", err.Error())
		}
	}()

	return fn()
}
