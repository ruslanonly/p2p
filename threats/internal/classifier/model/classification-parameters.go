package model

import "net"

type ClassificationParameters struct {
	SrcIP                  net.IP
	Duration               float32
	ProtocolType           string
	Service                string
	Flag                   string
	SrcBytes               float32
	DstBytes               float32
	Land                   int
	WrongFragment          int
	Urgent                 int
	Hot                    int
	NumFailedLogins        int
	LoggedIn               int
	NumCompromised         int
	RootShell              int
	SuAttempted            int
	NumRoot                int
	NumFileCreations       int
	NumShells              int
	NumAccessFiles         int
	NumOutboundCmds        int
	IsHostLogin            int
	IsGuestLogin           int
	Count                  float32
	SrvCount               float32
	SerrorRate             float32
	SrvSerrorRate          float32
	RerrorRate             float32
	SrvRerrorRate          float32
	SameSrvRate            float32
	DiffSrvRate            float32
	SrvDiffHostRate        float32
	DstHostCount           float32
	DstHostSrvCount        float32
	DstHostSameSrvRate     float32
	DstHostDiffSrvRate     float32
	DstHostSameSrcPortRate float32
	DstHostSrvDiffHostRate float32
	DstHostSerrorRate      float32
	DstHostSrvSerrorRate   float32
	DstHostRerrorRate      float32
	DstHostSrvRerrorRate   float32
}
