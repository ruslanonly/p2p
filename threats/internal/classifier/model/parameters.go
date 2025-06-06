package model

import "net"

type TrafficParameters struct {
	SrcIP                   net.IP
	DestinationPort         float32
	FlowDuration            float32
	TotalFwdPackets         float32
	TotalBackwardPackets    float32
	TotalLengthOfFwdPackets float32
	TotalLengthOfBwdPackets float32
	FwdPacketLengthMax      float32
	FwdPacketLengthMin      float32
	FwdPacketLengthMean     float32
	FwdPacketLengthStd      float32
	BwdPacketLengthMax      float32
	BwdPacketLengthMin      float32
	BwdPacketLengthMean     float32
	BwdPacketLengthStd      float32
	FlowBytesPerSecond      float32
	FlowPacketsPerSecond    float32
	FlowIATMean             float32
	FlowIATStd              float32
	FlowIATMax              float32
	FlowIATMin              float32
	FwdIATTotal             float32
	FwdIATMean              float32
	FwdIATStd               float32
	FwdIATMax               float32
	FwdIATMin               float32
	BwdIATTotal             float32
	BwdIATMean              float32
	BwdIATStd               float32
	BwdIATMax               float32
	BwdIATMin               float32
	FwdPSHFlags             float32
	// BwdPSHFlags             float32
	FwdURGFlags float32
	// BwdURGFlags             float32
	FwdHeaderLength      float32
	BwdHeaderLength      float32
	FwdPacketsPerSecond  float32
	BwdPacketsPerSecond  float32
	MinPacketLength      float32
	MaxPacketLength      float32
	PacketLengthMean     float32
	PacketLengthStd      float32
	PacketLengthVariance float32
	FinFlagCount         float32
	SynFlagCount         float32
	RstFlagCount         float32
	PshFlagCount         float32
	AckFlagCount         float32
	UrgFlagCount         float32
	CweFlagCount         float32
	EceFlagCount         float32
	DownUpRatio          float32
	AveragePacketSize    float32
	AvgFwdSegmentSize    float32
	AvgBwdSegmentSize    float32
	FwdHeaderLength1     float32
	// FwdAvgBytesBulk         float32
	// FwdAvgPacketsBulk       float32
	// FwdAvgBulkRate          float32
	// BwdAvgBytesBulk         float32
	// BwdAvgPacketsBulk       float32
	// BwdAvgBulkRate          float32
	SubflowFwdPackets    float32
	SubflowFwdBytes      float32
	SubflowBwdPackets    float32
	SubflowBwdBytes      float32
	InitWinBytesForward  float32
	InitWinBytesBackward float32
	ActDataPktFwd        float32
	MinSegSizeForward    float32
	ActiveMean           float32
	ActiveStd            float32
	ActiveMax            float32
	ActiveMin            float32
	IdleMean             float32
	IdleStd              float32
	IdleMax              float32
	IdleMin              float32
}

func (p TrafficParameters) Vectorize() []float32 {
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
