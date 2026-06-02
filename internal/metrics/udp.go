// Package metrics provides Prometheus metrics collection.
package metrics

// RecordUDPPacketReceived records a UDP packet received.
func (m *Metrics) RecordUDPPacketReceived(remoteAddr string) {
	if m.registry == nil {
		return
	}
	m.UDPPacketsReceived.WithLabelValues(remoteAddr).Inc()
}

// RecordUDPPacketSent records a UDP packet sent.
func (m *Metrics) RecordUDPPacketSent(remoteAddr string) {
	if m.registry == nil {
		return
	}
	m.UDPPacketsSent.WithLabelValues(remoteAddr).Inc()
}
