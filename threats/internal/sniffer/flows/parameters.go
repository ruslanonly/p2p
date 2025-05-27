package flows

import (
	"fmt"
	"math"
	"pkg/timestamp"
	"time"

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

func computeIAT(timestamps []timestamp.DateTimeNanoseconds) (total, mean, std, max, min float64) {
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

const idleThreshold = 1 * time.Second

func calculateActiveIdleFeatures(timestamps []time.Time) (activeStats, idleStats Stats) {
	activeStats = Stats{
		Mean: 0,
		Std:  0,
		Max:  0,
		Min:  0,
	}
	idleStats = Stats{
		Mean: 0,
		Std:  0,
		Max:  0,
		Min:  0,
	}

	if len(timestamps) < 2 {
		return
	}

	var activeDurations []float64
	var idleDurations []float64

	const maxReasonableDelta = 60.0

	for i := 1; i < len(timestamps); i++ {
		delta := timestamps[i].Sub(timestamps[i-1]).Seconds()
		if delta > maxReasonableDelta {
			continue
		}
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

func computeStats(data []float64) Stats {
	stats := Stats{}
	if len(data) == 0 {
		return stats
	}

	var sum, sumSq float64
	stats.Min = data[0]
	stats.Max = data[0]

	for _, v := range data {
		sum += v
		sumSq += v * v
		if v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}

	stats.Mean = sum / float64(len(data))

	variance := sumSq/float64(len(data)) - stats.Mean*stats.Mean
	if variance < 0 {
		variance = 0
	}
	stats.Std = math.Sqrt(variance)

	return stats
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
	// ip, _ := ipLayer.(*layers.IPv4)

	// if isLocalIP(ip.SrcIP) {
	// 	return true
	// } else {
	// 	return false
	// }
	return true
}

func safeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0.0
	}
	return f
}
