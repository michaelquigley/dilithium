package influx

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/openziti/dilithium/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"time"
)

func loadWestworld31Metrics(root string, retimeMs int64, client influxdb2.Client) error {
	peer := westworld3PeerId(root)
	writeApi := client.WriteAPI("", influxDbDatabase)
	for _, dataset := range westworld31Datasets {
		datasetPath := filepath.Join(root, dataset+".csv")
		data, err := util.ReadSamples(datasetPath)
		if err != nil {
			return errors.Wrapf(err, "error reading dataset [%s]", datasetPath)
		}
		for ts, v := range data {
			t := time.Unix(0, ts)
			if retimeMs > 0 {
				t = t.Add(time.Duration(retimeMs) * time.Millisecond)
			}
			p := influxdb2.NewPoint(dataset, nil, map[string]interface{}{"v": v}, t).AddTag("type", "westworld31").AddTag("peer", peer)
			writeApi.WritePoint(p)
		}
		logrus.Infof("wrote [%d] points for westworld3.1 peer [%s] dataset [%s]", len(data), peer, dataset)
	}

	return nil
}

func findWestworld31LatestTimestamp(root string) (time.Time, error) {
	peers := []*peer{
		&peer{
			id:    westworld3PeerId(root),
			paths: []string{root},
		},
	}
	return findLatestTimestamp(peers, westworld31Datasets)
}

func westworld3PeerId(root string) string {
	return filepath.Base(root)
}

var westworld31Datasets = []string{
	"tx_bytes",
	"tx_msgs",
	"retx_bytes",
	"retx_msgs",
	"rx_bytes",
	"rx_msgs",
	"tx_ack_bytes",
	"tx_ack_msgs",
	"rx_ack_bytes",
	"rx_ack_msgs",
	"tx_keepalive_bytes",
	"tx_keepalive_msgs",
	"rx_keepalive_bytes",
	"rx_keepalive_msgs",
	"tx_portal_capacity",
	"tx_portal_sz",
	"tx_portal_rx_sz",
	"retx_ms",
	"retx_scale",
	"dup_acks",
	"rx_portal_sz",
	"dup_rx_bytes",
	"dup_rx_msgs",
	"allocations",
	"errors",
}
