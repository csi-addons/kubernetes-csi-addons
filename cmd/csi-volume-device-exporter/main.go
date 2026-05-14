/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main provides the csi-volume-device-exporter entrypoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/discovery"
	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/monitoring/metrics"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	var (
		listenAddr      = flag.String("listen-address", ":9091", "Address to listen on for metrics and healthz")
		pollInterval    = flag.Duration("poll-interval", 30*time.Second, "Interval between discovery cycles")
		logLevel        = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		hostProc        = flag.String("host-proc", "/host/proc", "Path to host /proc mount inside container")
		hostSys         = flag.String("host-sys", "/host/sys", "Path to host /sys mount inside container")
		hostKubelet     = flag.String("host-kubelet", "/host/kubelet", "Path to host kubelet root mount inside container")
		realKubeletRoot = flag.String("kubelet-root", "/var/lib/kubelet", "Actual kubelet root path on the host (for mountinfo matching)")
		hostTrident     = flag.String("host-trident-tracking", "/host/trident/tracking", "Path to host Trident tracking dir inside container")
		showVersion     = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("csi-volume-device-exporter %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	logger := setupLogger(*logLevel)
	logger.Info("starting csi-volume-device-exporter",
		"version", version, "commit", commit)

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		logger.Error("NODE_NAME environment variable is required")
		os.Exit(1)
	}

	for _, p := range []struct{ name, val string }{
		{"host-proc", *hostProc},
		{"host-sys", *hostSys},
		{"host-kubelet", *hostKubelet},
		{"kubelet-root", *realKubeletRoot},
		{"host-trident-tracking", *hostTrident},
	} {
		clean := filepath.Clean(p.val)
		if !filepath.IsAbs(clean) || strings.Contains(clean, "..") {
			logger.Error("path must be absolute and must not escape via ..",
				"flag", p.name, "value", p.val)
			os.Exit(1)
		}
	}

	discoverers := []discovery.Discoverer{
		discovery.NewTridentDiscoverer(*hostTrident, *hostSys, nodeName, logger),
		discovery.NewHPEDiscoverer(*hostKubelet, *hostSys, nodeName, logger),
		discovery.NewKubeletDiscoverer(*hostKubelet, *realKubeletRoot, *hostProc, *hostSys, nodeName, logger),
	}

	m := metrics.New()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.Registry(), promhttp.HandlerOpts{}))

	var (
		lastSuccessTime time.Time
		lastSuccessMu   sync.RWMutex
	)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		lastSuccessMu.RLock()
		t := lastSuccessTime
		lastSuccessMu.RUnlock()

		if t.IsZero() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, "no successful discovery yet\n")
			return
		}
		if time.Since(t) > 2*(*pollInterval) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "last successful discovery: %s ago\n", time.Since(t))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok\n")
	})

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		logger.Info("received shutdown signal")
		cancel()
	}()

	go func() {
		logger.Info("starting HTTP server", "address", *listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error, initiating shutdown", "error", err)
			cancel()
		}
	}()

	logger.Info("starting discovery loop",
		"poll_interval", pollInterval.String(),
		"node", nodeName)

	run(ctx, discoverers, m, logger, &lastSuccessTime, &lastSuccessMu, *pollInterval)

	shutdown(server, logger)
}

func run(ctx context.Context, discoverers []discovery.Discoverer, m *metrics.Metrics, logger *slog.Logger, lastSuccess *time.Time, mu *sync.RWMutex, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runDiscovery(ctx, discoverers, m, logger, lastSuccess, mu)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runDiscovery(ctx, discoverers, m, logger, lastSuccess, mu)
		}
	}
}

func runDiscovery(ctx context.Context, discoverers []discovery.Discoverer, m *metrics.Metrics, logger *slog.Logger, lastSuccess *time.Time, mu *sync.RWMutex) {
	if ctx.Err() != nil {
		return
	}

	volumes := make(map[string]discovery.VolumeDevice)
	totalErrors := 0
	totalSuccesses := 0

	for _, d := range discoverers {
		start := time.Now()
		results, err := d.Discover(ctx)
		duration := time.Since(start)

		m.ObserveDiscoveryDuration(d.Name(), duration.Seconds())

		if err != nil {
			totalErrors++
			m.IncDiscoveryErrors(d.Name())
			logger.Warn("discovery error", "discoverer", d.Name(), "error", err)
			continue
		}

		totalSuccesses++
		for _, v := range results {
			if v.VolumeHandle == "" || v.Device == "" {
				continue
			}
			if existing, exists := volumes[v.VolumeHandle]; exists {
				if existing.Device != v.Device {
					logger.Warn("conflicting device for volume_handle, keeping first",
						"volume_handle", v.VolumeHandle,
						"kept_device", existing.Device,
						"kept_discoverer", existing.Driver,
						"ignored_device", v.Device,
						"ignored_discoverer", v.Driver,
					)
				}
				continue
			}
			volumes[v.VolumeHandle] = v
		}
	}

	if totalSuccesses > 0 {
		m.Reconcile(volumes)
		mu.Lock()
		m.SetLastSuccessfulNow()
		*lastSuccess = time.Now()
		mu.Unlock()
	} else {
		logger.Error("all discoverers failed, skipping reconcile and health update",
			"discoverer_count", len(discoverers))
	}

	logger.Debug("discovery cycle complete",
		"volumes_found", len(volumes), "errors", totalErrors, "successes", totalSuccesses)
}

func shutdown(server *http.Server, logger *slog.Logger) {
	logger.Info("shutting down HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
		logger.Warn("unknown log level, defaulting to info", "level", level)
		return logger
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
