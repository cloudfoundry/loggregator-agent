package api

import (
	"fmt"
	"net/http"
)

var (
	pathTemplate = "%s/internal/v4/syslog_drain_urls?batch_size=%d&next_id=%d"
)

type Client struct {
	Client    *http.Client
	Addr      string
	BatchSize int
}

func (w Client) Get(nextID int) (*http.Response, error) {
	return w.Client.Get(fmt.Sprintf(pathTemplate, w.Addr, w.BatchSize, nextID))
}
