package v1

import (
	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"log"
	"net"

	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
)

type ByteArrayWriter interface {
	Write(message []byte)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metrics.MetricOption) (metrics.Counter, error)
}

type NetworkReader struct {
	connection  net.PacketConn
	writer      ByteArrayWriter
	rxMsgCount  metrics.Counter
	contextName string
	buffer      *diodes.OneToOne
}

func NewNetworkReader(
	address string,
	writer ByteArrayWriter,
	m MetricClient,
) (*NetworkReader, error) {
	connection, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}
	log.Printf("udp bound to: %s", connection.LocalAddr())
	//TODO: err handling
	rxErrCount, err := m.NewCounter("dropped")
	rxMsgCount, err := m.NewCounter("ingress")

	return &NetworkReader{
		connection: connection,
		rxMsgCount: rxMsgCount,
		writer:     writer,
		buffer: diodes.NewOneToOne(10000, gendiodes.AlertFunc(func(missed int) {
			log.Printf("network reader dropped messages %d", missed)
			rxErrCount.Add(float64(missed))
		})),
	}, nil
}

func (nr *NetworkReader) StartReading() {
	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, _, err := nr.connection.ReadFrom(readBuffer)
		if err != nil {
			log.Printf("Error while reading: %s", err)
			return
		}
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])

		nr.buffer.Set(readData)
	}
}

func (nr *NetworkReader) StartWriting() {
	for {
		data := nr.buffer.Next()
		nr.rxMsgCount.Add(1)
		nr.writer.Write(data)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
