// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pflock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithLockRunsFn(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	ran := false
	err := WithLock(pfPath, func() error {
		ran = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ran, "guarded function must run")
}

func TestWithLockPropagatesFnError(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	boom := errors.New("boom")
	err := WithLock(pfPath, func() error { return boom })
	assert.ErrorIs(t, err, boom, "function error must propagate")
}

func TestWithLockRemovesLockFile(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	require.NoError(t, WithLock(pfPath, func() error { return nil }))
	_, statErr := os.Stat(pfPath + ".lock")
	assert.ErrorIs(t, statErr, os.ErrNotExist, "lock file must be removed after unlock")
}

func TestWithLockRemovesLockFileOnFnError(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	boom := errors.New("boom")
	err := WithLock(pfPath, func() error { return boom })
	assert.ErrorIs(t, err, boom, "function error must propagate")
	_, statErr := os.Stat(pfPath + ".lock")
	assert.ErrorIs(t, statErr, os.ErrNotExist, "lock file must be removed even when fn fails")
}

func TestWithLockTimeoutWhenLocked(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	holder := flock.New(pfPath + ".lock")
	require.NoError(t, holder.Lock(), "test precondition: hold the lock")
	defer func() { _ = holder.Unlock() }()
	err := WithLockTimeout(pfPath, 200*time.Millisecond, func() error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire lock")
}

// TestLockSchedulePinsTheValuesTheFileLockWaits: the default wait and the re-probe interval are the numbers callers plan around, so pin both.
func TestLockSchedulePinsTheValuesTheFileLockWaits(t *testing.T) {
	assert.Equal(t, 5*time.Second, defaultWait(), "WithLock must default to a 5 second wait")
	assert.Equal(t, 100*time.Millisecond, retryInterval(), "the lock must be re-probed every 100ms while a competing run holds it")
}

func TestWithLockIgnoresLeftoverLockFile(t *testing.T) {
	pfPath := filepath.Join(t.TempDir(), "projectfile.yaml")
	require.NoError(t, os.WriteFile(pfPath+".lock", nil, 0o600), "test precondition: a killed run left its lock file")
	ran := false
	require.NoError(t, WithLockTimeout(pfPath, 200*time.Millisecond, func() error { ran = true; return nil }))
	assert.True(t, ran, "a lock file no process holds must not block")
}
