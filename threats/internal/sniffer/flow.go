package sniffer

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"threats/internal/classifier/model"
	"time"

	"github.com/CN-TU/go-flows/flows"
	"github.com/CN-TU/go-ipfix"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func meanAndStd(values []int) (float64, float64) {
	n := len(values)
	if n == 0 {
		return 0, 0
	}
	var sum, sqSum float64
	for _, v := range values {
		f := float64(v)
		sum += f
		sqSum += f * f
	}
	mean := sum / float64(n)
	variance := sqSum/float64(n) - mean*mean
	return mean, sqrtSafe(variance)
}

func sqrtSafe(x float64) float64 {
	if x < 0 {
		return 0
	}
	return float64(math.Sqrt(x))
}

func minInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	min := slice[0]
	for _, v := range slice {
		if v < min {
			min = v
		}
	}
	return min
}

func maxInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	max := slice[0]
	for _, v := range slice {
		if v > max {
			max = v
		}
	}
	return max
}

func computeIAT(timestamps []flows.DateTimeNanoseconds) (total, mean, std, max, min float64) {
	n := len(timestamps)
	if n < 2 {
		return 0, 0, 0, 0, 0
	}
	var diffs []float64
	for i := 1; i < n; i++ {
		delta := float64(timestamps[i] - timestamps[i-1])
		diffs = append(diffs, delta)
	}

	sum, sqSum := 0.0, 0.0
	minVal, maxVal := diffs[0], diffs[0]
	for _, d := range diffs {
		sum += d
		sqSum += d * d
		if d < minVal {
			minVal = d
		}
		if d > maxVal {
			maxVal = d
		}
	}
	mean = sum / float64(len(diffs))
	variance := sqSum/float64(len(diffs)) - mean*mean
	std = sqrtSafe(variance)

	return sum, mean, std, maxVal, minVal
}

const idleThreshold = time.Second

func calculateActiveIdleFeatures(timestamps []time.Time) (activeStats, idleStats *Stats) {
	if len(timestamps) < 2 {
		return &Stats{}, &Stats{}
	}

	var activeDurations []float64
	var idleDurations []float64

	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i].Sub(timestamps[i-1]).Seconds()

		if delta <= idleThreshold.Seconds() {
			activeDurations = append(activeDurations, delta)
		} else {
			idleDurations = append(idleDurations, delta)
		}
	}

	activeStats = computeStats(activeDurations)
	idleStats = computeStats(idleDurations)

	return
}

type Stats struct {
	Mean float64
	Std  float64
	Max  float64
	Min  float64
}

func computeStats(data []float64) *Stats {
	if len(data) == 0 {
		return &Stats{Mean: 0, Std: 0, Max: 0, Min: 0}
	}

	var sum, sumSq float64
	min := data[0]
	max := data[0]

	for _, v := range data {
		sum += v
		sumSq += v * v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	mean := sum / float64(len(data))
	variance := (sumSq / float64(len(data))) - (mean * mean)
	std := math.Sqrt(variance)

	return &Stats{
		Mean: mean,
		Std:  std,
		Max:  max,
		Min:  min,
	}
}

type LengthStats struct {
	Min  int
	Max  int
	Mean float64
	Std  float64
	Var  float64
}

func computeLengthStats(lengths []int) LengthStats {
	if len(lengths) == 0 {
		return LengthStats{Mean: 0, Std: 0, Max: 0, Min: 0, Var: 0}
	}
	sum := 0.0
	min := lengths[0]
	max := lengths[0]

	for _, l := range lengths {
		sum += float64(l)
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}
	mean := sum / float64(len(lengths))

	// Variance and std
	var sumSq float64
	for _, l := range lengths {
		diff := float64(l) - mean
		sumSq += diff * diff
	}
	variance := sumSq / float64(len(lengths))
	std := math.Sqrt(variance)

	return LengthStats{
		Min:  min,
		Max:  max,
		Mean: mean,
		Var:  variance,
		Std:  std,
	}
}

func computePacketsPerSecond(timestamps []time.Time) float64 {
	if len(timestamps) < 2 {
		return 0
	}
	duration := timestamps[len(timestamps)-1].Sub(timestamps[0]).Seconds()
	if duration <= 0 {
		return 0
	}
	return float64(len(timestamps)) / duration
}

// Простая генерация ключа на основе IP-адресов и портов
func GenerateKey(packet gopacket.Packet) string {
	net := packet.NetworkLayer()
	trans := packet.TransportLayer()
	if net == nil || trans == nil {
		return "unknown"
	}
	src := net.NetworkFlow().Src().String()
	dst := net.NetworkFlow().Dst().String()
	sport := trans.TransportFlow().Src().String()
	dport := trans.TransportFlow().Dst().String()
	return fmt.Sprintf("%s:%s-%s:%s", src, sport, dst, dport)
}

func DetermineDirection(packet gopacket.Packet) bool {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return true
	}
	ip, _ := ipLayer.(*layers.IPv4)

	if isLocalIP(ip.SrcIP) {
		return true
	} else {
		return false
	}
}

func IPFixDateNanosecondsArrToTimeArr(arr []ipfix.DateTimeNanoseconds) []time.Time {
	out := make([]time.Time, len(arr))
	for i, ts := range arr {
		out[i] = time.Unix(0, int64(ts))
	}

	return out
}

type PacketEvent struct {
	timestamp flows.DateTimeNanoseconds
	key       string
	lowToHigh bool
	window    uint64
	eventNr   uint64
	length    int

	Packet gopacket.Packet
}

func (e *PacketEvent) Timestamp() flows.DateTimeNanoseconds { return e.timestamp }
func (e *PacketEvent) Key() string                          { return e.key }
func (e *PacketEvent) LowToHigh() bool                      { return e.lowToHigh }
func (e *PacketEvent) SetWindow(w uint64)                   { e.window = w }
func (e *PacketEvent) Window() uint64                       { return e.window }
func (e *PacketEvent) EventNr() uint64                      { return e.eventNr }
func (e *PacketEvent) Length() int                          { return e.length }

type CICIDSFlow struct {
	flows.BaseFlow

	handler func(*model.TrafficParameters)

	table *flows.FlowTable

	startTime flows.DateTimeNanoseconds
	endTime   flows.DateTimeNanoseconds

	allTimestamps      []flows.DateTimeNanoseconds
	forwardTimestamps  []flows.DateTimeNanoseconds
	backwardTimestamps []flows.DateTimeNanoseconds

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

func NewCICIDSFlow(
	e flows.Event,
	table *flows.FlowTable,
	key string,
	active bool,
	ctx *flows.EventContext,
	id uint64,
	handler func(*model.TrafficParameters),
) flows.Flow {
	flow := &CICIDSFlow{
		handler: handler,
	}

	flow.Init(table, key, active, ctx, id)
	flow.forwardMin = math.MaxInt
	flow.minSegSizeForward = math.MaxInt

	return flow
}

func (f *CICIDSFlow) Event(e flows.Event, ctx *flows.EventContext) {
	pkt, ok := e.(*PacketEvent)
	if !ok {
		return
	}

	fmt.Println("EVENT")

	if f.startTime == 0 {
		f.startTime = pkt.Timestamp()
	}

	if f.srcIP == nil {
		packet := pkt.Packet
		if netLayer := packet.NetworkLayer(); netLayer != nil {
			f.srcIP = net.ParseIP(netLayer.NetworkFlow().Src().String())
		}
		if trans := packet.TransportLayer(); trans != nil {
			if port, err := strconv.Atoi(trans.TransportFlow().Dst().String()); err == nil {
				f.dstPort = port
			}
		}
	}

	tcpLayer := pkt.Packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)

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

	f.endTime = pkt.Timestamp()
	length := pkt.Length()
	timestamp := pkt.Timestamp()
	f.allTimestamps = append(f.allTimestamps, timestamp)

	if pkt.LowToHigh() {
		f.forwardPackets++
		f.forwardBytes += pkt.length
		f.forwardPacketLengths = append(f.forwardPacketLengths, pkt.length)
		f.forwardTimestamps = append(f.forwardTimestamps, pkt.Timestamp())

		if tcpLayer != nil {
			tcp := tcpLayer.(*layers.TCP)
			if tcp.PSH {
				f.fwdPSHFlags++
			}
			if tcp.URG {
				f.fwdURGFlags++
			}
			f.fwdHeaderLength += int(tcp.DataOffset * 4) // в байтах

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

		if length > f.forwardMax {
			f.forwardMax = length
		}
		if length < f.forwardMin {
			f.forwardMin = length
		}

	} else {
		f.backwardPackets++
		f.backwardBytes += pkt.length
		f.backwardPacketLengths = append(f.backwardPacketLengths, pkt.length)
		f.backwardTimestamps = append(f.backwardTimestamps, pkt.Timestamp())

		if tcpLayer != nil {
			tcp := tcpLayer.(*layers.TCP)
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

	f.Export(flows.FlowEndReasonIdle, ctx, flows.DateTimeNanoseconds(time.Now().UnixNano()))
}

func (f *CICIDSFlow) Export(reason flows.FlowEndReason, ctx *flows.EventContext, now flows.DateTimeNanoseconds) {
	fmt.Println("EXPORTING")
	duration := float64(f.endTime-f.startTime) / 1e9

	fwdMean, fwdStd := meanAndStd(f.forwardPacketLengths)
	bwdMean, bwdStd := meanAndStd(f.backwardPacketLengths)

	totalBytes := f.forwardBytes + f.backwardBytes
	totalPackets := f.forwardPackets + f.backwardPackets

	bytesPerSec := float64(0)
	packetsPerSec := float64(0)

	if duration > 0 {
		bytesPerSec = float64(totalBytes) / duration
		packetsPerSec = float64(totalPackets) / duration
	}

	_, flowIATMean, flowIATStd, flowIATMax, flowIATMin := computeIAT(f.allTimestamps)
	fwdIATTotal, fwdIATMean, fwdIATStd, fwdIATMax, fwdIATMin := computeIAT(f.forwardTimestamps)
	bwdIATTotal, bwdIATMean, bwdIATStd, bwdIATMax, bwdIATMin := computeIAT(f.backwardTimestamps)

	lengthStats := computeLengthStats(append(f.forwardPacketLengths, f.backwardPacketLengths...))

	fwdPPS := computePacketsPerSecond(IPFixDateNanosecondsArrToTimeArr(f.forwardTimestamps))
	bwdPPS := computePacketsPerSecond(IPFixDateNanosecondsArrToTimeArr(f.backwardTimestamps))

	activeStats, idleStats := calculateActiveIdleFeatures(IPFixDateNanosecondsArrToTimeArr(f.allTimestamps))

	features := model.TrafficParameters{
		SrcIP:                   f.srcIP,
		DestinationPort:         float32(f.dstPort),
		FlowDuration:            float32(duration * 1e9), // обратно в наносекунды
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
		ActiveMean: float32(activeStats.Mean),
		ActiveStd:  float32(activeStats.Std),
		ActiveMax:  float32(activeStats.Max),
		ActiveMin:  float32(activeStats.Min),
		IdleMean:   float32(idleStats.Mean),
		IdleStd:    float32(idleStats.Std),
		IdleMax:    float32(idleStats.Max),
		IdleMin:    float32(idleStats.Min),
	}

	f.handler(&features)
	f.Stop()
}
