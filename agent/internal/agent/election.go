package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftnet "github.com/libp2p/go-libp2p-raft"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/protocols/defaultproto"
	defaultprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/defaultproto/messages"
	"github.com/ruslanonly/agent/internal/consensus/hubelection"
	"github.com/ruslanonly/agent/internal/fsm"
)

func (a *Agent) organizeSegmentHubElection() []peer.ID {
	_, abonents := a.getSplittedPeers()

	peerIDs := make([]peer.ID, 0)

	if len(abonents) < 1 {
		return nil
	} else {
		var abonent AgentPeerInfo
		for _, a := range abonents {
			abonent = a
			break
		}

		var message defaultprotomessages.Message
		if len(abonents) == 1 {
			// Абонент становится хабом сразу, если он единственный абонент в сегменте
			log.Printf("Отправка сообщения о необходимости стать единственным хабом")
			message = defaultprotomessages.Message{
				Type: defaultprotomessages.BecomeOnlyOneHubMessageType,
			}
		} else {
			// Первый абонент из списка абонентов должен являться инициатором выборов среди другого сегмента, о котором он знает
			log.Printf("Отправка сообщения о необходимости инициализировать выборы среди абонентов сегмента")

			_, abonents := a.getSplittedPeers()
			abonentsPeerInfos := make([]defaultprotomessages.InfoAboutSegmentPeerInfo, 0)

			for peerID, peerInfo := range abonents {
				addrs := a.node.PeerAddrs(peerID)
				peerIDs = append(peerIDs, peerID)

				abonentsPeerInfos = append(abonentsPeerInfos, defaultprotomessages.InfoAboutSegmentPeerInfo{
					ID:    peerID,
					IsHub: peerInfo.status.IsHub(),
					Addrs: addrs,
				})
			}

			body := defaultprotomessages.InitializeElectionRequestMessageBody{
				Peers: abonentsPeerInfos,
			}

			if marshaledBody, err := json.Marshal(body); err != nil {
				log.Println("Ошибка при маршалинге тела InitializeElectionRequestMessageBody:", err)
				return nil
			} else {
				message = defaultprotomessages.Message{
					Type: defaultprotomessages.InitializeElectionRequestMessageType,
					Body: marshaledBody,
				}
			}

		}

		s, err := a.node.Host.NewStream(context.Background(), abonent.ID, defaultproto.ProtocolID)
		if err != nil {
			log.Println(err)
			return nil
		}

		if err := json.NewEncoder(s).Encode(message); err != nil {
			log.Println("Ошибка при отправке запроса:", err)
			return nil
		}

		s.Close()
	}

	return peerIDs
}

func (a *Agent) prepareForElection(segmentPeers []AgentPeerInfoPeer) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(a.node.Host.ID().String())
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "raft",
		Level: hclog.Error,
	})

	store := raft.NewInmemStore()
	logStore := raft.NewInmemStore()
	snapshotStore := raft.NewDiscardSnapshotStore()

	transport, err := raftnet.NewLibp2pTransport(a.node.Host, 10*time.Second)
	if err != nil {
		log.Printf("Возникла ошибка при подготовке transport для выборов нового хаба: %v", err)
		return
	}

	raftNode, err := raft.NewRaft(config, &hubelection.HubElectionRaftFSM{}, logStore, store, snapshotStore, transport)
	if err != nil {
		log.Printf("Возникла ошибка при подготовке raftNode для выборов нового хаба: %v", err)
		return
	}

	var servers []raft.Server
	for _, segmentPeer := range segmentPeers {
		if len(segmentPeer.Addrs) > 0 {
			servers = append(servers, raft.Server{
				ID:      raft.ServerID(segmentPeer.ID.String()),
				Address: raft.ServerAddress(segmentPeer.Addrs[0]),
			})
		}
	}

	cfg := raft.Configuration{
		Servers: servers,
	}

	raftNode.BootstrapCluster(cfg)

	observerlCn := make(chan raft.Observation, 1)
	obs := raft.NewObserver(
		observerlCn,
		false,
		func(o *raft.Observation) bool {
			_, ok := o.Data.(raft.LeaderObservation)
			return ok
		},
	)

	raftNode.RegisterObserver(obs)
	defer raftNode.DeregisterObserver(obs)

	for {
		select {
		case obsEvent := <-observerlCn:
			if leaderObs, ok := obsEvent.Data.(raft.LeaderObservation); ok {
				if leaderObs.LeaderID == raft.ServerID(a.node.Host.ID().String()) {
					fmt.Println("👑 Я выбран хабом")
					a.fsm.Event(fsm.BecameHubAfterElectionFSMEvent)
				} else {
					leaderPeerID, err := peer.Decode(string(leaderObs.LeaderID))
					if err != nil {
						log.Println("❌ Не удалось декодировать LeaderID в peer.ID:", leaderObs.LeaderID, err)
					} else {
						leaderAddrs := a.node.Host.Peerstore().Addrs(leaderPeerID)
						fmt.Println("👑 Я не выбран хабом. Теперь мой хаб он:", leaderAddrs, leaderPeerID)
						a.fsm.Event(fsm.BecameAbonentAfterElectionFSMEvent, leaderAddrs[0].String(), leaderPeerID.String())
					}
				}

				time.Sleep(5 * time.Second)
				raftNode.Shutdown()

				return
			}
		}
	}
}

func (a *Agent) initializeElectionForMySegment(segmentPeers []AgentPeerInfoPeer) {
	segmentPeerInfos := make([]defaultprotomessages.InfoAboutSegmentPeerInfo, 0)

	for _, peerInfo := range segmentPeers {
		addrs := a.node.PeerAddrs(peerInfo.ID)

		segmentPeerInfos = append(segmentPeerInfos, defaultprotomessages.InfoAboutSegmentPeerInfo{
			ID:    peerInfo.ID,
			IsHub: peerInfo.status.IsHub(),
			Addrs: addrs,
		})
	}

	body := defaultprotomessages.ElectionRequestMessageBody{
		Peers: segmentPeerInfos,
	}

	if marshaledBody, err := json.Marshal(body); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {
		infoAboutSegmentMessage := defaultprotomessages.Message{
			Type: defaultprotomessages.ElectionRequestMessageType,
			Body: marshaledBody,
		}

		if marshalledMessage, err := json.Marshal(infoAboutSegmentMessage); err != nil {
			log.Printf("Ошибка при маршаллинге сообщения: %v\n", err)
		} else {
			for _, p := range segmentPeers {
				if p.ID == a.node.Host.ID() {
					continue
				}

				stream, err := a.node.Host.NewStream(context.Background(), p.ID, defaultproto.ProtocolID)
				if err != nil {
					log.Println(err)
					return
				}

				if _, err := stream.Write(append(marshalledMessage, '\n')); err != nil {
					log.Println("Ошибка при отправке запроса:", err)
					return
				}

				stream.Close()
			}
		}
	}

	a.prepareForElection(segmentPeers)
}
