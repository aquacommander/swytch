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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/swytchdb/engine/beacon"
	"github.com/swytchdb/swytch/metrics"
)

// startEffectsWithMaxMemory runs the --maxmemory parsing and engine startup
// steps of Main and returns the configured absolute bytes and the reported
// effective limit.
func startEffectsWithMaxMemory(t *testing.T, spec string) (configured, effective int64) {
	t.Helper()

	maxMemory, maxMemoryPct, err := beacon.ParseMemoryLimit(spec)
	if err != nil {
		t.Fatalf("ParseMemoryLimit(%q): %v", spec, err)
	}

	if err := InitializeEffects(&EffectsConfig{
		MemoryLimit:        maxMemory,
		MemoryLimitPercent: maxMemoryPct,
		Port:               6379,
	}); err != nil {
		t.Fatalf("InitializeEffects: %v", err)
	}
	t.Cleanup(StopCluster)

	return maxMemory, effectiveMemoryLimit(maxMemory, GetActiveEngine())
}

// gatherMemoryMaxBytes exports the adapter through the metrics collector and
// returns the swytch_redis_memory_max_bytes gauge.
func gatherMemoryMaxBytes(t *testing.T, memoryLimit int64) float64 {
	t.Helper()

	s, err := NewServer(DefaultServerConfig())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() {
		s.cancel()
		s.handler.Close()
	})

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewCollector(NewMetricsAdapter(s.Stats(), s.Handler(), memoryLimit)))

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	for _, mf := range families {
		if mf.GetName() == "swytch_redis_memory_max_bytes" {
			return mf.GetMetric()[0].GetGauge().GetValue()
		}
	}

	t.Fatal("swytch_redis_memory_max_bytes not exported")

	return 0
}

func TestEffectiveMemoryLimit_Percentage(t *testing.T) {
	configured, effective := startEffectsWithMaxMemory(t, "85%")

	target := GetActiveEngine().MemoryTarget()
	if configured != 0 || target <= 0 {
		t.Fatalf("precondition: configured = %d, engine target = %d; want 0 and > 0", configured, target)
	}

	if effective != target {
		t.Fatalf("effective limit = %d, want engine target %d", effective, target)
	}

	if got, want := formatBytes(uint64(effective)), formatBytes(uint64(target)); got != want {
		t.Fatalf("banner Max Memory = %s, want %s", got, want)
	}

	if got := gatherMemoryMaxBytes(t, effective); got != float64(target) {
		t.Fatalf("swytch_redis_memory_max_bytes = %v, want %d", got, target)
	}
}

func TestEffectiveMemoryLimit_Absolute(t *testing.T) {
	const want = int64(128 << 20)

	configured, effective := startEffectsWithMaxMemory(t, "128mb")
	if configured != want {
		t.Fatalf("configured = %d, want %d", configured, want)
	}

	if effective != want {
		t.Fatalf("effective limit = %d, want %d", effective, want)
	}

	if got := formatBytes(uint64(effective)); got != "128.00MB" {
		t.Fatalf("banner Max Memory = %s, want 128.00MB", got)
	}

	if got := gatherMemoryMaxBytes(t, effective); got != float64(want) {
		t.Fatalf("swytch_redis_memory_max_bytes = %v, want %d", got, want)
	}
}

func TestEffectiveMemoryLimit_NoEngine(t *testing.T) {
	const want = int64(64 << 20)

	if got := effectiveMemoryLimit(want, nil); got != want {
		t.Fatalf("effectiveMemoryLimit(%d, nil) = %d, want %d", want, got, want)
	}
}
