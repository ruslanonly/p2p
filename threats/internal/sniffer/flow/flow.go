package flow

import (
	"math"
	"net"
	"strconv"
	"sync"

	"pkg/timestamp"
	"threats/internal/classifier/model"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type FlowKey struct {
	SrcIP, DstIP     string
	SrcPort, DstPort string
	Protocol         string
}

func ExtractKeyFromPacket(packet gopacket.Packet) FlowKey {
	net := packet.NetworkLayer()
	trans := packet.TransportLayer()

	if net == nil || trans == nil {
		return FlowKey{
			SrcIP:   "0",
			SrcPort: "0",
			DstIP:   "0",
			DstPort: "0",
		}
	}

	srcIP := net.NetworkFlow().Src().String()
	dstIP := net.NetworkFlow().Dst().String()
	srcPort := trans.TransportFlow().Src().String()
	destPort := trans.TransportFlow().Dst().String()

	return FlowKey{
		SrcIP:   srcIP,
		SrcPort: srcPort,
		DstIP:   dstIP,
		DstPort: destPort,
	}
}

type FlowEvent struct {
	Key    FlowKey
	Packet gopacket.Packet
}

func (evt *FlowEvent) isForward() bool {
	net := evt.Packet.NetworkLayer()
	trans := evt.Packet.TransportLayer()

	if net == nil || trans == nil {
		return true
	}

	srcIP := net.NetworkFlow().Src().String()
	dstIP := net.NetworkFlow().Dst().String()
	srcPort := trans.TransportFlow().Src().String()
	destPort := trans.TransportFlow().Dst().String()

	if evt.Key.SrcIP == srcIP && evt.Key.SrcPort == srcPort &&
		evt.Key.DstIP == dstIP && evt.Key.DstPort == destPort {
		return true
	}

	return true
}

type Flow struct {
	startTime timestamp.DateTimeNanoseconds
	endTime   timestamp.DateTimeNanoseconds

	allTimestamps      []timestamp.DateTimeNanoseconds
	forwardTimestamps  []timestamp.DateTimeNanoseconds
	backwardTimestamps []timestamp.DateTimeNanoseconds

	srcIP                 net.IP
	dstPort               int
	forwardPacketLengths  []int
	backwardPacketLengths []int
	forwardPackets        int
	backwardPackets       int
	forwardBytes          int
	backwardBytes         int
	forwardMax            int
	forwardMin            int

	finFlagCount int
	synFlagCount int
	rstFlagCount int
	pshFlagCount int
	ackFlagCount int
	urgFlagCount int
	cweFlagCount int
	eceFlagCount int

	fwdPSHFlags     int
	bwdPSHFlags     int
	fwdURGFlags     int
	bwdURGFlags     int
	fwdHeaderLength int
	bwdHeaderLength int

	initWinBytesFwd   int
	initWinBytesBwd   int
	actDataPktFwd     int
	minSegSizeForward int
}

func NewFlow() *Flow {
	return &Flow{
		forwardMin:        math.MaxInt,
		minSegSizeForward: math.MaxInt,
	}
}

func (f *Flow) AddPacket(evt FlowEvent) {
	pkt := evt.Packet
	length := evt.Packet.Metadata().Length
	timestamp := timestamp.FromTimeDateTimeNanoseconds(evt.Packet.Metadata().Timestamp)
	f.endTime = timestamp
	f.allTimestamps = append(f.allTimestamps, timestamp)

	if f.startTime == 0 {
		f.startTime = timestamp
	}

	if f.srcIP == nil {
		if netLayer := pkt.NetworkLayer(); netLayer != nil {
			f.srcIP = net.ParseIP(netLayer.NetworkFlow().Src().String())
		}
		if trans := pkt.TransportLayer(); trans != nil {
			if port, err := strconv.Atoi(trans.TransportFlow().Dst().String()); err == nil {
				f.dstPort = port
			}
		}
	}

	tcpLayer := pkt.Layer(layers.LayerTypeTCP)
	var tcp *layers.TCP
	if tcpLayer != nil {
		tcp = tcpLayer.(*layers.TCP)
		if tcp.FIN {
			f.finFlagCount++
		}
		if tcp.SYN {
			f.synFlagCount++
		}
		if tcp.RST {
			f.rstFlagCount++
		}
		if tcp.PSH {
			f.pshFlagCount++
		}
		if tcp.ACK {
			f.ackFlagCount++
		}
		if tcp.URG {
			f.urgFlagCount++
		}
		if tcp.CWR {
			f.cweFlagCount++
		}
		if tcp.ECE {
			f.eceFlagCount++
		}
	}

	if length > f.forwardMax {
		f.forwardMax = length
	}
	if length < f.forwardMin {
		f.forwardMin = length
	}

	if evt.isForward() {
		f.forwardPackets++
		f.forwardBytes += length
		f.forwardPacketLengths = append(f.forwardPacketLengths, length)
		f.forwardTimestamps = append(f.forwardTimestamps, timestamp)

		if tcp != nil {
			if tcp.PSH {
				f.fwdPSHFlags++
			}
			if tcp.URG {
				f.fwdURGFlags++
			}
			f.fwdHeaderLength += int(tcp.DataOffset * 4)
			if f.forwardPackets == 1 {
				f.initWinBytesFwd = int(tcp.Window)
			}
			if len(tcp.Payload) > 0 {
				f.actDataPktFwd++
			}
			if len(tcp.Payload) < f.minSegSizeForward {
				f.minSegSizeForward = len(tcp.Payload)
			}
		}
	} else {
		f.backwardPackets++
		f.backwardBytes += length
		f.backwardPacketLengths = append(f.backwardPacketLengths, length)
		f.backwardTimestamps = append(f.backwardTimestamps, timestamp)

		if tcp != nil {
			if tcp.PSH {
				f.bwdPSHFlags++
			}
			if tcp.URG {
				f.bwdURGFlags++
			}
			f.bwdHeaderLength += int(tcp.DataOffset * 4)
			if f.backwardPackets == 1 {
				f.initWinBytesBwd = int(tcp.Window)
			}
		}
	}

	f.Export()
}

func (f *Flow) Export() model.TrafficParameters {
	durationInSeconds := float64(f.endTime-f.startTime) / 1e9

	fwdMean, fwdStd := meanAndStd(f.forwardPacketLengths)
	bwdMean, bwdStd := meanAndStd(f.backwardPacketLengths)

	totalBytes := f.forwardBytes + f.backwardBytes
	totalPackets := f.forwardPackets + f.backwardPackets

	bytesPerSec := float64(0)
	packetsPerSec := float64(0)

	if durationInSeconds > 0 {
		bytesPerSec = float64(totalBytes) / durationInSeconds
		packetsPerSec = float64(totalPackets) / durationInSeconds
	}

	_, flowIATMean, flowIATStd, flowIATMax, flowIATMin := computeIAT(f.allTimestamps)
	fwdIATTotal, fwdIATMean, fwdIATStd, fwdIATMax, fwdIATMin := computeIAT(f.forwardTimestamps)
	bwdIATTotal, bwdIATMean, bwdIATStd, bwdIATMax, bwdIATMin := computeIAT(f.backwardTimestamps)

	lengthStats := computeLengthStats(append(f.forwardPacketLengths, f.backwardPacketLengths...))

	fwdPPS := computePacketsPerSecond(timestamp.DateTimeNanosecondsArrToTimeArr(f.forwardTimestamps))
	bwdPPS := computePacketsPerSecond(timestamp.DateTimeNanosecondsArrToTimeArr(f.backwardTimestamps))

	activeStats, idleStats := calculateActiveIdleFeatures(timestamp.DateTimeNanosecondsArrToTimeArr(f.allTimestamps))

	features := model.TrafficParameters{
		SrcIP:                   f.srcIP,
		DestinationPort:         float32(f.dstPort),
		FlowDuration:            float32(durationInSeconds * 1e9),
		TotalFwdPackets:         float32(f.forwardPackets),
		TotalBackwardPackets:    float32(f.backwardPackets),
		TotalLengthOfFwdPackets: float32(f.forwardBytes),
		TotalLengthOfBwdPackets: float32(f.backwardBytes),
		FwdPacketLengthMax:      float32(f.forwardMax),
		FwdPacketLengthMin:      float32(f.forwardMin),
		FwdPacketLengthMean:     float32(fwdMean),
		FwdPacketLengthStd:      float32(fwdStd),
		BwdPacketLengthMax:      float32(maxInt(f.backwardPacketLengths)),
		BwdPacketLengthMin:      float32(minInt(f.backwardPacketLengths)),
		BwdPacketLengthMean:     float32(bwdMean),
		BwdPacketLengthStd:      float32(bwdStd),
		FlowBytesPerSecond:      float32(bytesPerSec),
		FlowPacketsPerSecond:    float32(packetsPerSec),
		FlowIATMean:             float32(flowIATMean),
		FlowIATStd:              float32(flowIATStd),
		FlowIATMax:              float32(flowIATMax),
		FlowIATMin:              float32(flowIATMin),
		FwdIATTotal:             float32(fwdIATTotal),
		FwdIATMean:              float32(fwdIATMean),
		FwdIATStd:               float32(fwdIATStd),
		FwdIATMax:               float32(fwdIATMax),
		FwdIATMin:               float32(fwdIATMin),
		BwdIATTotal:             float32(bwdIATTotal),
		BwdIATMean:              float32(bwdIATMean),
		BwdIATStd:               float32(bwdIATStd),
		BwdIATMax:               float32(bwdIATMax),
		BwdIATMin:               float32(bwdIATMin),
		FwdPSHFlags:             float32(f.fwdPSHFlags),
		// BwdPSHFlags:             float32(f.bwdPSHFlags),
		FwdURGFlags: float32(f.fwdURGFlags),
		// BwdURGFlags:             float32(f.bwdURGFlags),
		FwdHeaderLength:      float32(f.fwdHeaderLength),
		BwdHeaderLength:      float32(f.bwdHeaderLength),
		FwdPacketsPerSecond:  float32(fwdPPS),
		BwdPacketsPerSecond:  float32(bwdPPS),
		MinPacketLength:      float32(lengthStats.Min),
		MaxPacketLength:      float32(lengthStats.Max),
		PacketLengthMean:     float32(lengthStats.Mean),
		PacketLengthStd:      float32(lengthStats.Std),
		PacketLengthVariance: float32(lengthStats.Var),
		FinFlagCount:         float32(f.finFlagCount),
		SynFlagCount:         float32(f.synFlagCount),
		RstFlagCount:         float32(f.rstFlagCount),
		PshFlagCount:         float32(f.pshFlagCount),
		AckFlagCount:         float32(f.ackFlagCount),
		UrgFlagCount:         float32(f.urgFlagCount),
		CweFlagCount:         float32(f.cweFlagCount),
		EceFlagCount:         float32(f.eceFlagCount),
		DownUpRatio: func() float32 {
			if f.forwardPackets == 0 {
				return 0
			}
			return float32(f.backwardPackets) / float32(f.forwardPackets)
		}(),
		AveragePacketSize: func() float32 {
			if totalPackets == 0 {
				return 0
			}
			return float32(totalBytes) / float32(totalPackets)
		}(),
		AvgFwdSegmentSize: func() float32 {
			if f.forwardPackets == 0 {
				return 0
			}
			return float32(f.forwardBytes) / float32(f.forwardPackets)
		}(),
		AvgBwdSegmentSize: func() float32 {
			if f.backwardPackets == 0 {
				return 0
			}
			return float32(f.backwardBytes) / float32(f.backwardPackets)
		}(),
		FwdHeaderLength1: 0,
		// FwdAvgBytesBulk:      0,
		// FwdAvgPacketsBulk:    0,
		// FwdAvgBulkRate:       0,
		// BwdAvgBytesBulk:      0,
		// BwdAvgPacketsBulk:    0,
		// BwdAvgBulkRate:       0,
		SubflowFwdPackets:    float32(f.forwardPackets),
		SubflowFwdBytes:      float32(f.forwardBytes),
		SubflowBwdPackets:    float32(f.backwardPackets),
		SubflowBwdBytes:      float32(f.backwardBytes),
		InitWinBytesForward:  float32(f.initWinBytesFwd),
		InitWinBytesBackward: float32(f.initWinBytesBwd),
		ActDataPktFwd:        float32(f.actDataPktFwd),
		MinSegSizeForward: func() float32 {
			if f.minSegSizeForward == math.MaxInt {
				return 0
			}
			return float32(f.minSegSizeForward)
		}(),
		ActiveMean: float32(safeFloat(activeStats.Mean)),
		ActiveStd:  float32(safeFloat(activeStats.Std)),
		ActiveMax:  float32(safeFloat(activeStats.Max)),
		ActiveMin:  float32(safeFloat(activeStats.Min)),
		IdleMean:   float32(safeFloat(idleStats.Mean)),
		IdleStd:    float32(safeFloat(idleStats.Std)),
		IdleMax:    float32(safeFloat(idleStats.Max)),
		IdleMin:    float32(safeFloat(idleStats.Min)),
	}

	return features
}

type FlowsManager struct {
	flows map[FlowKey]*Flow
	mu    sync.Mutex

	exporter func(*model.TrafficParameters)
}

func NewFlowsManager(exporter func(*model.TrafficParameters)) *FlowsManager {
	return &FlowsManager{
		flows:    make(map[FlowKey]*Flow),
		exporter: exporter,
	}
}

func (m *FlowsManager) AddEvent(evt FlowEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := ExtractKeyFromPacket(evt.Packet)

	flow, exists := m.flows[key]
	if !exists {
		flow = NewFlow()
		m.flows[key] = flow
	} else {
		flow.Export()
		features := flow.Export()
		m.exporter(&features)
	}

	flow.AddPacket(evt)
}
