package syslog

import (
	"errors"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
)

type metrics interface {
	NewCounter(string) func(uint64)
}

type WriterFactory struct {
	m metrics
}

func NewWriterFactory(m metrics) WriterFactory {
	return WriterFactory{
		m: m,
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
			f.m.NewCounter("Egress"),
		), nil
	case "syslog":
		return NewTCPWriter(
			urlBinding,
			netConf,
			skipCertVerify,
			f.m.NewCounter("Egress"),
		), nil
	case "syslog-tls":
		return NewTLSWriter(
			urlBinding,
			netConf,
			skipCertVerify,
			f.m.NewCounter("Egress"),
		), nil
	default:
		return nil, errors.New("unsupported protocol")
	}
}
