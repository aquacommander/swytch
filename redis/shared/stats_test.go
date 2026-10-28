/*
 * Copyright 2026 Swytch Labs BV
 *
 * This file is part of Swytch.
 *
 * Swytch is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of
 * the License, or (at your option) any later version.
 *
 * Swytch is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Swytch. If not, see <https://www.gnu.org/licenses/>.
 */
package shared

import (
	"sync"
	"testing"
	"time"
)

func TestStatsResetKeepsLiveGauges(t *testing.T) {
	s := NewStats()

	s.ConnectionOpened()
	s.ConnectionOpened()
	s.ClientBlocked()
	s.RecordCommand(CmdGet, 10, false)

	s.Reset()

	if got := s.CurrConnections.Load(); got != 2 {
		t.Fatalf("CurrConnections after Reset = %d, want 2", got)
	}

	if got := s.BlockedClients.Load(); got != 1 {
		t.Fatalf("BlockedClients after Reset = %d, want 1", got)
	}

	if got := s.TotalConnections.Load(); got != 0 {
		t.Fatalf("TotalConnections after Reset = %d, want 0", got)
	}

	if got := len(s.GetCommandStats()); got != 0 {
		t.Fatalf("command stats after Reset has %d entries, want 0", got)
	}

	s.ConnectionClosed()
	s.ConnectionClosed()
	s.ClientUnblocked()

	if got := s.CurrConnections.Load(); got != 0 {
		t.Fatalf("CurrConnections after closing = %d, want 0", got)
	}

	if got := s.BlockedClients.Load(); got != 0 {
		t.Fatalf("BlockedClients after unblocking = %d, want 0", got)
	}
}

// TestStatsLatencyPercentilesConcurrent exercises percentile reads alongside
// latency recording; run with -race to check the ring index is read under lock.
func TestStatsLatencyPercentilesConcurrent(t *testing.T) {
	s := NewStats()

	const n = 1000

	var wg sync.WaitGroup

	wg.Go(func() {
		for range n {
			s.RecordGetLatency(time.Microsecond)
			s.RecordSetLatency(time.Microsecond)
			s.RecordCommand(CmdGet, 1, false)
		}
	})

	wg.Go(func() {
		for range n {
			_ = s.Snapshot()
		}
	})

	wg.Wait()

	if got := s.GetLatencyP99(); got != time.Microsecond {
		t.Fatalf("GetLatencyP99 = %v, want %v", got, time.Microsecond)
	}
}
