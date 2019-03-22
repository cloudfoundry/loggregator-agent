package syslog

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"errors"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
)

type metricClient interface {
	NewCounter(name string, opts ...metrics.MetricOption) (metrics.Counter, error)
}

type WriterFactory struct {
	egressMetric metrics.Counter
}

func NewWriterFactory(m metricClient) WriterFactory {
	// TODO: err checking
	egressMetric, _ := m.NewCounter("egress")
	return WriterFactory{
		egressMetric: egressMetric,
	}
}

func (f WriterFactory) NewWriter(
	urlBinding *URLBinding,
	netConf NetworkTimeoutConfig,
	skipCertVerify bool,
) (egress.WriteCloser, error) {
	switch urlBinding.URL.Scheme {
	case "https":
		return NewHTTPSWriter(
			urlBinding,
			netConf,
			skipCertVerify,
			f.egressMetric,
		), nil
	case "syslog":
		return NewTCPWriter(
			urlBinding,
			netConf,
			skipCertVerify,
			f.egressMetric,
		), nil
	case "syslog-tls":
		return NewTLSWriter(
			urlBinding,
			netConf,
			skipCertVerify,
			f.egressMetric,
		), nil
	default:
		return nil, errors.New("unsupported protocol")
	}
}
