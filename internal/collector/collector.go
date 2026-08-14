// Package collector implements the prometheus.Collector interface for
// monerod state.
//
// On each scrape, Refresh is called first to populate the cached
// RPC responses, then Collect is called by the prometheus client
// library.
package collector

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/rossigee/monero-exporter/internal/rpc"
)

// Collector is a thread-safe prometheus.Collector that holds the latest
// snapshot of monerod state.
type Collector struct {
	client *rpc.Client
	mu     sync.RWMutex
	info   *rpc.GetInfoResult
	header *rpc.BlockHeader
	log    *logrus.Logger

	lastScrapeErr error
	scrapeAt      time.Time
}

// New constructs a Collector bound to client.
func New(client *rpc.Client, log *logrus.Logger) *Collector {
	return &Collector{client: client, log: log}
}

// Register mounts the collector on the global prometheus registry.
// Returns the live collector so callers can invoke [Refresh] on
// /metrics hits.
func Register(client *rpc.Client, log *logrus.Logger) (*Collector, error) {
	c := New(client, log)
	if err := prometheus.Register(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Refresh updates the cached monerod state. Called from the HTTP
// handler before each scrape so that Collect is essentially constant time.
func (c *Collector) Refresh(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, ierr := c.client.GetInfo(ctx)
	if ierr != nil {
		c.lastScrapeErr = ierr
		return
	}
	header, herr := c.client.GetLastBlockHeader(ctx)
	if herr != nil {
		c.lastScrapeErr = herr
		return
	}
	c.info = info
	c.header = header
	c.lastScrapeErr = nil
	c.scrapeAt = time.Now()
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descs() {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	info := c.info
	header := c.header
	scrapeAt := c.scrapeAt
	err := c.lastScrapeErr
	c.mu.RUnlock()

	// Always emit the "up" gauge so a stale daemon is visible.
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("monero_up", "1 if the last scrape of monerod succeeded", nil, nil),
		prometheus.GaugeValue, boolToFloat64(err == nil),
	)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("monero_scrape_error", "1 if the last scrape failed", nil, nil),
			prometheus.GaugeValue, 1,
		)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("monero_scrape_error", "1 if the last scrape failed", nil, nil),
		prometheus.GaugeValue, 0,
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("monero_scrape_timestamp_seconds", "unix timestamp of last successful scrape", nil, nil),
		prometheus.GaugeValue, float64(scrapeAt.Unix()),
	)

	emitInfo(ch, info)
	emitHeader(ch, header)
}

func (c *Collector) descs() []*prometheus.Desc {
	d := make([]*prometheus.Desc, 0, 3+len(infoDescs())+len(headerDescs()))
	d = append(d,
		prometheus.NewDesc("monero_up", "1 if the last scrape succeeded", nil, nil),
		prometheus.NewDesc("monero_scrape_error", "1 if the last scrape failed", nil, nil),
		prometheus.NewDesc("monero_scrape_timestamp_seconds", "unix timestamp of last successful scrape", nil, nil),
	)
	d = append(d, infoDescs()...)
	d = append(d, headerDescs()...)
	return d
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
