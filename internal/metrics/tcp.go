// Package metrics provides Prometheus metrics collection.
package metrics

// UpdateTCPConnections updates the number of active TCP connections.
func (m *Metrics) UpdateTCPConnections(count int) {
	if m.registry == nil {
		return
	}
	m.TCPConnections.Set(float64(count))
}

// RecordTCPBytesReceived records bytes received via TCP.
func (m *Metrics) RecordTCPBytesReceived(remoteAddr string, bytes int) {
	if m.registry == nil {
		return
	}
	m.TCPBytesReceived.WithLabelValues(remoteAddr).Add(float64(bytes))
}

// RecordTCPBytesSent records bytes sent via TCP.
func (m *Metrics) RecordTCPBytesSent(remoteAddr string, bytes int) {
	if m.registry == nil {
		return
	}
	m.TCPBytesSent.WithLabelValues(remoteAddr).Add(float64(bytes))
}
