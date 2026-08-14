package collector

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/rossigee/monero-exporter/internal/rpc"
)

// headerDescs returns metric descriptors emitted from get_last_block_header.
func headerDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		prometheus.NewDesc("monero_lastblock_height", "height of the last known block", nil, nil),
		prometheus.NewDesc("monero_lastblock_difficulty", "cumulative difficulty of the last block", nil, nil),
		prometheus.NewDesc("monero_lastblock_reward", "total reward (subsidy + fees) of the last block", nil, nil),
		prometheus.NewDesc("monero_lastblock_major_version", "major block-version of the last block", nil, nil),
		prometheus.NewDesc("monero_lastblock_minor_version", "minor block-version of the last block", nil, nil),
		prometheus.NewDesc("monero_lastblock_timestamp_seconds", "unix time of the last block", nil, nil),
		prometheus.NewDesc("monero_lastblock_transactions", "transaction count in the last block", nil, nil),
	}
}

func emitHeader(ch chan<- prometheus.Metric, h *rpc.BlockHeader) {
	if h == nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(headerDescs()[0], prometheus.GaugeValue, float64(h.Height))
	ch <- prometheus.MustNewConstMetric(headerDescs()[1], prometheus.GaugeValue, float64(h.Difficulty))
	ch <- prometheus.MustNewConstMetric(headerDescs()[2], prometheus.GaugeValue, float64(h.Reward))
	ch <- prometheus.MustNewConstMetric(headerDescs()[3], prometheus.GaugeValue, float64(h.MajorVersion))
	ch <- prometheus.MustNewConstMetric(headerDescs()[4], prometheus.GaugeValue, float64(h.MinorVersion))
	ch <- prometheus.MustNewConstMetric(headerDescs()[5], prometheus.GaugeValue, float64(h.Timestamp))
	ch <- prometheus.MustNewConstMetric(headerDescs()[6], prometheus.GaugeValue, float64(h.TxCount))
}
