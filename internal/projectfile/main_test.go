// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"os"
	"testing"

	"kiota.ch/projectfile/core/v2/internal/netfetch"
)

func TestMain(m *testing.M) {
	restore := netfetch.NoBackoff()
	code := m.Run()
	restore()
	os.Exit(code)
}
