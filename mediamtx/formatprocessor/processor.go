// Package formatprocessor cleans and normalizes streams.
package formatprocessor

import (
	"fearpro13/h265_transcoder/mediamtx/unit"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// Processor cleans and normalizes streams.
type Processor interface {
	// process a Unit.
	ProcessUnit(unit.Unit) error

	// process a RTP packet and convert it into a unit.
	ProcessRTPPacket(
		pkt *rtp.Packet,
		ntp time.Time,
		pts time.Duration,
		hasNonRTSPReaders bool,
	) (Unit, error)
}

// New allocates a Processor.
func New(
	udpMaxPayloadSize int,
	forma format.Format,
	generateRTPPackets bool,
) (Processor, error) {
	switch forma := forma.(type) {
	case *format.H265:
		return newH265(udpMaxPayloadSize, forma, generateRTPPackets)

	case *format.H264:
		return newH264(udpMaxPayloadSize, forma, generateRTPPackets)

	default:
		return newGeneric(udpMaxPayloadSize, forma, generateRTPPackets)
	}
}
