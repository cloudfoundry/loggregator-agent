package syslog

import (
	"errors"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
)

type metrics interface {
	NewCounter(string) func(uint64)
}

type WriterFactory struct {
	egressMetric func(uint64)
}

func IsValidSyslogScheme(scheme string) bool {
	switch scheme {
	case "https":
		fallthrough
	case "syslog":
		fallthrough
	case "syslog-tls":
		return true
	default:
		return false
	}
}

func NewWriterFactory(m metrics) WriterFactory {
	return WriterFactory{
		egressMetric: m.NewCounter("Egress"),
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
