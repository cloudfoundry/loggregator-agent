package syslog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
)

type HTTPSWriter struct {
	hostname     string
	appID        string
	url          *url.URL
	client       *http.Client
	egressMetric func(delta uint64)
}

func NewHTTPSWriter(
	binding *URLBinding,
	netConf NetworkTimeoutConfig,
	skipCertVerify bool,
	egressMetric func(delta uint64),
) egress.WriteCloser {

	client := httpClient(netConf, skipCertVerify)

	return &HTTPSWriter{
		url:          binding.URL,
		appID:        binding.AppID,
		hostname:     binding.Hostname,
		client:       client,
		egressMetric: egressMetric,
	}
}

func (w *HTTPSWriter) Write(env *loggregator_v2.Envelope) error {
	msgs := generateRFC5424Messages(env, w.hostname, w.appID)
	for _, msg := range msgs {
		b, err := msg.MarshalBinary()
		if err != nil {
			return err
		}

		resp, err := w.client.Post(w.url.String(), "text/plain", bytes.NewBuffer(b))
		if err != nil {
			return w.sanitizeError(w.url, err)
		}
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("Syslog Writer: Post responded with %d status code", resp.StatusCode)
		}
	}
	w.egressMetric(1)

	return nil
}

func (*HTTPSWriter) sanitizeError(u *url.URL, err error) error {
	if u == nil || u.User == nil {
		return err
	}

	if user := u.User.Username(); user != "" {
		err = errors.New(strings.Replace(err.Error(), user, "<REDACTED>", -1))
	}

	if p, ok := u.User.Password(); ok {
		err = errors.New(strings.Replace(err.Error(), p, "<REDACTED>", -1))
	}
	return err
}

func (*HTTPSWriter) Close() error {
	return nil
}

func httpClient(netConf NetworkTimeoutConfig, skipCertVerify bool) *http.Client {
	tlsConfig := plumbing.NewTLSConfig()
	tlsConfig.InsecureSkipVerify = skipCertVerify

	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   netConf.DialTimeout,
			KeepAlive: netConf.Keepalive,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}
}
