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
package redis

import (
	"testing"
	"time"
)

// TestConfigResetStatKeepsConnectionCount checks that CONFIG RESETSTAT leaves
// the live connection gauge alone, so it does not go negative as clients that
// were open during the reset disconnect.
func TestConfigResetStatKeepsConnectionCount(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	defer func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop Redis test server: %v", err)
		}
	}()

	stats := server.Stats()

	a := newRedisClient(t, server.Addr().String())
	defer a.close()

	b := newRedisClient(t, server.Addr().String())

	// A reply proves each connection has been registered.
	if resp := a.ping(); resp != "PONG" {
		t.Fatalf("PING a: got %q", resp)
	}

	if resp := b.ping(); resp != "PONG" {
		t.Fatalf("PING b: got %q", resp)
	}

	a.sendCommand("CONFIG", "RESETSTAT")

	if resp := a.readReply(); resp != "OK" {
		t.Fatalf("CONFIG RESETSTAT: got %q", resp)
	}

	if got := stats.CurrConnections.Load(); got != 2 {
		t.Fatalf("CurrConnections after RESETSTAT = %d, want 2", got)
	}

	if got := stats.TotalConnections.Load(); got != 0 {
		t.Fatalf("TotalConnections after RESETSTAT = %d, want 0", got)
	}

	b.close()

	deadline := time.Now().Add(5 * time.Second)
	for stats.CurrConnections.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := stats.CurrConnections.Load(); got != 1 {
		t.Fatalf("CurrConnections after one client closed = %d, want 1", got)
	}
}
