package classifier

import (
	"threats/internal/classifier/model"
)

type Aggregator struct{}

func NewAggregator() (*Aggregator, error) {
	aggregator := &Aggregator{}

	return aggregator, nil
}

func (e *Aggregator) Vectorize(p model.TrafficParameters) []float32 {

	inputVector := []float32{
		p.DestinationPort,
		p.FlowDuration,
		p.TotalFwdPackets,
		p.TotalBackwardPackets,
		p.TotalLengthOfFwdPackets,
		p.TotalLengthOfBwdPackets,
		p.FwdPacketLengthMax,
		p.FwdPacketLengthMin,
		p.FwdPacketLengthMean,
		p.FwdPacketLengthStd,
		p.BwdPacketLengthMax,
		p.BwdPacketLengthMin,
		p.BwdPacketLengthMean,
		p.BwdPacketLengthStd,
		p.FlowBytesPerSecond,
		p.FlowPacketsPerSecond,
		p.FlowIATMean,
		p.FlowIATStd,
		p.FlowIATMax,
		p.FlowIATMin,
		p.FwdIATTotal,
		p.FwdIATMean,
		p.FwdIATStd,
		p.FwdIATMax,
		p.FwdIATMin,
		p.BwdIATTotal,
		p.BwdIATMean,
		p.BwdIATStd,
		p.BwdIATMax,
		p.BwdIATMin,
		p.FwdPSHFlags,
		p.FwdURGFlags,
		p.FwdHeaderLength,
		p.BwdHeaderLength,
		p.FwdPacketsPerSecond,
		p.BwdPacketsPerSecond,
		p.MinPacketLength,
		p.MaxPacketLength,
		p.PacketLengthMean,
		p.PacketLengthStd,
		p.PacketLengthVariance,
		p.FinFlagCount,
		p.SynFlagCount,
		p.RstFlagCount,
		p.PshFlagCount,
		p.AckFlagCount,
		p.UrgFlagCount,
		p.CweFlagCount,
		p.EceFlagCount,
		p.DownUpRatio,
		p.AveragePacketSize,
		p.AvgFwdSegmentSize,
		p.AvgBwdSegmentSize,
		p.FwdHeaderLength1,
		p.SubflowFwdPackets,
		p.SubflowFwdBytes,
		p.SubflowBwdPackets,
		p.SubflowBwdBytes,
		p.InitWinBytesForward,
		p.InitWinBytesBackward,
		p.ActDataPktFwd,
		p.MinSegSizeForward,
		p.ActiveMean,
		p.ActiveStd,
		p.ActiveMax,
		p.ActiveMin,
		p.IdleMean,
		p.IdleStd,
		p.IdleMax,
		p.IdleMin,
	}

	return inputVector
}
