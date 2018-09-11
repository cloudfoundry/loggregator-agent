package v2

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

type counterID struct {
	name     string
	tagsHash string
}

type CounterAggregator struct {
	writer        Writer
	counterTotals map[counterID]uint64
}

func NewCounterAggregator(w Writer) *CounterAggregator {
	return &CounterAggregator{
		writer:        w,
		counterTotals: make(map[counterID]uint64),
	}
}

func (ca *CounterAggregator) Write(msgs []*loggregator_v2.Envelope) error {
	for i := range msgs {
		c := msgs[i].GetCounter()
		if c != nil {
			if len(ca.counterTotals) > 10000 {
				ca.resetTotals()
			}

			id := counterID{
				name:     c.Name,
				tagsHash: hashTags(msgs[i].GetTags()),
			}

			if c.GetTotal() != 0 {
				ca.counterTotals[id] = c.GetTotal()
				continue
			}

			ca.counterTotals[id] = ca.counterTotals[id] + c.GetDelta()
			c.Total = ca.counterTotals[id]
		}
	}

	return ca.writer.Write(msgs)
}

func (ca *CounterAggregator) resetTotals() {
	ca.counterTotals = make(map[counterID]uint64)
}

func hashTags(tags map[string]string) string {
	hash := ""
	elements := []mapElement{}
	for k, v := range tags {
		elements = append(elements, mapElement{k, v})
	}
	sort.Sort(byKey(elements))
	for _, element := range elements {
		kHash, vHash := sha1.New(), sha1.New()
		io.WriteString(kHash, element.k)
		io.WriteString(vHash, element.v)
		hash += fmt.Sprintf("%x%x", kHash.Sum(nil), vHash.Sum(nil))
	}
	return hash
}

type byKey []mapElement

func (a byKey) Len() int           { return len(a) }
func (a byKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byKey) Less(i, j int) bool { return a[i].k < a[j].k }

type mapElement struct {
	k, v string
}
