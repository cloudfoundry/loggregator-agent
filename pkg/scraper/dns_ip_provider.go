package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type record []string

type dns struct {
	Records []record
}

func NewDNSMetricUrlProvider(dnsFile string, port int) func() []string {
	return func() []string {
		file, err := os.Open(dnsFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		var d dns
		err = json.NewDecoder(file).Decode(&d)
		if err != nil {
			panic(err)
		}

		var metricAddrs []string
		for _, r := range d.Records {
			ip := r[0]
			metricAddrs = append(metricAddrs, fmt.Sprintf("http://%s:%d/metrics", ip, port))
		}

		return metricAddrs
	}
}
