package collector

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/rossigee/monero-exporter/internal/rpc"
)

// infoDescs returns the metric descriptors emitted from the daemon info
// payload. Names mirror the monero-exporter convention adopted by
// cirocosta/monero-exporter for consistency with existing dashboards.
func infoDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		prometheus.NewDesc("monero_info_height", "current chain height", nil, nil),
		prometheus.NewDesc("monero_info_target_height", "target chain height when in sync", nil, nil),
		prometheus.NewDesc("monero_info_tx_pool_size", "transactions in the mempool", nil, nil),
		prometheus.NewDesc("monero_info_block_size_limit_bytes", "maximum hard limit of a block", nil, nil),
		prometheus.NewDesc("monero_info_block_size_median_bytes", "rolling median for dynamic fee calc", nil, nil),
		prometheus.NewDesc("monero_info_offline", "1 if the node is offline", nil, nil),
		prometheus.NewDesc("monero_info_synchronized", "1 if the node is in sync with the network", nil, nil),
		prometheus.NewDesc("monero_info_mainnet", "1 if connected to mainnet", nil, nil),
		prometheus.NewDesc("monero_info_restricted", "1 if RPC is in restricted mode", nil, nil),
		prometheus.NewDesc("monero_info_incoming_connections", "inbound P2P connections", nil, nil),
		prometheus.NewDesc("monero_info_outgoing_connections", "outbound P2P connections", nil, nil),
		prometheus.NewDesc("monero_info_rpc_connections", "active RPC connections", nil, nil),
		prometheus.NewDesc("monero_info_database_size_bytes", "size of monerod's LMDB", nil, nil),
		prometheus.NewDesc("monero_info_free_space_bytes", "free space of monerod's data volume", nil, nil),
		prometheus.NewDesc("monero_info_start_time_seconds", "unix time when monerod started", nil, nil),
		prometheus.NewDesc("monero_info_uptime_seconds", "seconds since monerod started", nil, nil),
	}
}

func emitInfo(ch chan<- prometheus.Metric, info *rpc.GetInfoResult) {
	if info == nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(infoDescs()[0], prometheus.GaugeValue, float64(info.Height))
	ch <- prometheus.MustNewConstMetric(infoDescs()[1], prometheus.GaugeValue, float64(info.TargetHeight))
	ch <- prometheus.MustNewConstMetric(infoDescs()[2], prometheus.GaugeValue, float64(info.TxPoolSize))
	ch <- prometheus.MustNewConstMetric(infoDescs()[3], prometheus.GaugeValue, float64(info.BlockSizeLimit))
	ch <- prometheus.MustNewConstMetric(infoDescs()[4], prometheus.GaugeValue, float64(info.BlockSizeMedian))
	ch <- prometheus.MustNewConstMetric(infoDescs()[5], prometheus.GaugeValue, boolToFloat64(info.Offline))
	ch <- prometheus.MustNewConstMetric(infoDescs()[6], prometheus.GaugeValue, boolToFloat64(info.Synchronized))
	ch <- prometheus.MustNewConstMetric(infoDescs()[7], prometheus.GaugeValue, boolToFloat64(info.Mainnet))
	ch <- prometheus.MustNewConstMetric(infoDescs()[8], prometheus.GaugeValue, boolToFloat64(info.Restricted))
	ch <- prometheus.MustNewConstMetric(infoDescs()[9], prometheus.GaugeValue, float64(info.IncomingConnections))
	ch <- prometheus.MustNewConstMetric(infoDescs()[10], prometheus.GaugeValue, float64(info.OutgoingConnections))
	ch <- prometheus.MustNewConstMetric(infoDescs()[11], prometheus.GaugeValue, float64(info.RPCConnections))
	ch <- prometheus.MustNewConstMetric(infoDescs()[12], prometheus.GaugeValue, float64(info.DatabaseSize))
	ch <- prometheus.MustNewConstMetric(infoDescs()[13], prometheus.GaugeValue, float64(info.FreeSpace))
	ch <- prometheus.MustNewConstMetric(infoDescs()[14], prometheus.GaugeValue, float64(info.StartTime))

	uptime := float64(0)
	if info.StartTime > 0 {
		uptime = float64(time.Now().Unix() - info.StartTime)
	}
	ch <- prometheus.MustNewConstMetric(infoDescs()[15], prometheus.GaugeValue, uptime)
}
