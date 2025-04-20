package fsm

import (
	"context"
	"log"

	"github.com/looplab/fsm"
)

const (
	// S0 Начальное состоения
	IdleAgentFSMState   string = "Idle"
	// S1 Состояние подключения к хабу
	ConnectingToHubAgentFSMState   string = "ConnectingToHub"
	// S2 Состояние ожидания сообщений как абонент
	ListeningMessagesAsAbonentAgentFSMState   string = "ListeningMessagesAsAbonent"
	// S6 Состояние ожидания сообщений как хаб
	ListeningMessagesAsHubAgentFSMState   string = "ListeningMessagesAsHub"
	// S3 Состояние неопределенности без хаба (абонент не подключен)
	PendingNewHubAgentFSMState   string = "PendingNewHub"
	// S4 Состояние обработки сообщения как абонент
	HandlingMessageAsAbonentAgentFSMState   string = "HandlingMessageAsAbonent"
	// S9 Состояние распространения информации о подозрительном трафике
	PublishSuspiciosHostInfoAgentFSMState   string = "PublishSuspiciosHostInfo"
	// S5 Состояние выборов нового хаба
	ElectingNewHubAgentFSMState   string = "ElectingNewHub"
	// S7 Состояние ретрансляции полученных сообщений всей сети (хабам)
	NetworkBroadcastingAgentFSMState   string = "NetworkBroadcasting"
	// S8 Состояние подключения нового абонента
	ConnectingNewAbonentAgentFSMState   string = "ConnectingNewAbonent"
	// S10 Состояние достижения соглашения о блокировке подозрительного хоста
	BlockingSuspiciosHostAgentFSMState   string = "BlockingSuspiciosHost"
	// S11 Состояние ретрансляции сообщения абонентам из собственного сегмента
	AbonentsSegmentBroadcastAgentFSMState   string = "AbonentsSegmentBroadcast"
)

const (
	// E0 Событие о получении настроек подключения
	ReadInitialSettingsAgentFSMEvent   string = "ReadInitialSettings"
	// E1 Событие успешного подключения к хабу
	ConnectedToHubAgentFSMEvent   string = "ConnectedToHub"
	// E2 Событие неуспешного подключения к хабу
	NotConnectedToHubAgentFSMEvent   string = "NotConnectedToHub"
	// E3 Событие проверки работоспособности хаба
	CheckHubHeartbeatAgentFSMEvent   string = "CheckHubHeartbeat"
	// E4 Создание запроса на подключение от абонента к хабу
	RequestConnectionFromAbonentToHubAgentFSMEvent   string = "RequestConnectionFromAbonentToHub"
	// E5 Событие поступления обычного сообщения для абонента
	RegularMessageForAbonentAgentFSMEvent   string = "RegularMessageForAbonent"
	// E6 Событие обработки обычного сообщения для абонента
	RegularMessageForAbonentIsHandledAgentFSMEvent   string = "RegularMessageForAbonentIsHandled"
	// E7 Получена информация о подозрительном хосте
	SuspiciosHostInfoFSMEvent   string = "SuspiciosHostInfo"
	// E8 Информация о подозрительном хосте распространена
	SuspiciosHostInfoIsPublishedFSMEvent   string = "SuspiciosHostInfoIsPublished"
	// E9 Событие запроса выборов нового хаба
	ElectNewHubRequestFSMEvent   string = "ElectNewHubRequest"
	// E10 Выбран обычным абонентом после выборов (получил нового хаба)
	BecameAbonentAfterElectionFSMEvent   string = "BecameAbonentAfterElection"
	// E11 Выбран хабом после выборов
	BecameHubAfterElectionFSMEvent   string = "BecameHubAfterElection"
	// E12 Хаб больше не доступен после проверки
	HubDoNotHaveHeartbeatFSMEvent   string = "HubDoNotHaveHeartbeat"
	// E13 Сообщение от абонента своего сегмента
	MessageFromSegmentAbonentFSMEvent   string = "MessageFromSegmentAbonent"
	// E14 Ретрансляция сообщения от абонента закончилась
	MessageFromSegmentAbonentIsPublishedFSMEvent   string = "MessageFromSegmentAbonentIsPublished"
	// E15 Получение запрос на подключение от абонента
	ConnectionRequestToHubAgentFSMEvent   string = "ConnectionRequestToHub"
	// E16 Получилось/не получилось подключить нового абонента/хаба
	ConnectionRequestToHubIsHandledAgentFSMEvent   string = "ConnectionRequestToHubIsHandled"
	// E17 Сообщение о подозрительном хосте от абонента сегмента
	SuspiciosHostInfoFromSegmentAbonentAgentFSMEvent   string = "SuspiciosHostInfoFromSegmentAbonent"
	// E18 Подозрительный хост не заблокирован хабом
	HubDidNotBlockedSuspiciosHostAgentFSMEvent   string = "HubDidNotBlockedSuspiciosHost"
	// E19 Подозрительный хост заблокирован хабом
	HubBlockedSuspiciosHostAgentFSMEvent   string = "HubBlockedSuspiciosHost"
	// E20 Ретрансляция информация хабом закончилась
	HubPublishedInfoAgentFSMEvent   string = "HubBlockedSuspiciosHost"
)

type AgentFSM struct {
	Fsm *fsm.FSM
}

func NewAgentFSM() *AgentFSM {
	a := &AgentFSM{}
	a.Fsm = fsm.NewFSM(
		IdleAgentFSMState,
		fsm.Events{
			{Name: ReadInitialSettingsAgentFSMEvent, Src: []string{IdleAgentFSMState}, Dst: ConnectingToHubAgentFSMState},
			{Name: ConnectedToHubAgentFSMEvent, Src: []string{ConnectingToHubAgentFSMState}, Dst: ListeningMessagesAsAbonentAgentFSMState},
			{Name: NotConnectedToHubAgentFSMEvent, Src: []string{ConnectingToHubAgentFSMState, ListeningMessagesAsAbonentAgentFSMState}, Dst: PendingNewHubAgentFSMState},
			{Name: RequestConnectionFromAbonentToHubAgentFSMEvent, Src: []string{PendingNewHubAgentFSMState}, Dst: ConnectingToHubAgentFSMState},
			{Name: CheckHubHeartbeatAgentFSMEvent, Src: []string{ListeningMessagesAsAbonentAgentFSMState}, Dst: ListeningMessagesAsAbonentAgentFSMState},
			{Name: SuspiciosHostInfoFSMEvent, Src: []string{ListeningMessagesAsAbonentAgentFSMState}, Dst: PublishSuspiciosHostInfoAgentFSMState},
			{Name: SuspiciosHostInfoFSMEvent, Src: []string{ListeningMessagesAsHubAgentFSMState}, Dst: BlockingSuspiciosHostAgentFSMState},
			{Name: SuspiciosHostInfoIsPublishedFSMEvent, Src: []string{PublishSuspiciosHostInfoAgentFSMState}, Dst: ListeningMessagesAsAbonentAgentFSMState},
			{Name: RegularMessageForAbonentAgentFSMEvent, Src: []string{ListeningMessagesAsAbonentAgentFSMState}, Dst: HandlingMessageAsAbonentAgentFSMState},
			{Name: RegularMessageForAbonentIsHandledAgentFSMEvent, Src: []string{HandlingMessageAsAbonentAgentFSMState}, Dst: ListeningMessagesAsAbonentAgentFSMState},
			{Name: ElectNewHubRequestFSMEvent, Src: []string{ListeningMessagesAsAbonentAgentFSMState}, Dst: ElectingNewHubAgentFSMState},
			{Name: BecameAbonentAfterElectionFSMEvent, Src: []string{ElectingNewHubAgentFSMState}, Dst: ConnectingToHubAgentFSMState},
			{Name: BecameHubAfterElectionFSMEvent, Src: []string{ElectingNewHubAgentFSMState}, Dst: ListeningMessagesAsHubAgentFSMState},
			{Name: HubDoNotHaveHeartbeatFSMEvent, Src: []string{PendingNewHubAgentFSMState}, Dst: ElectingNewHubAgentFSMState},
			{Name: MessageFromSegmentAbonentFSMEvent, Src: []string{ListeningMessagesAsHubAgentFSMState}, Dst: NetworkBroadcastingAgentFSMState},
			{Name: MessageFromSegmentAbonentIsPublishedFSMEvent, Src: []string{NetworkBroadcastingAgentFSMState}, Dst: ListeningMessagesAsHubAgentFSMState},
			{Name: ConnectionRequestToHubAgentFSMEvent, Src: []string{ListeningMessagesAsHubAgentFSMState}, Dst: ConnectingNewAbonentAgentFSMState},
			{Name: ConnectionRequestToHubIsHandledAgentFSMEvent, Src: []string{ConnectingNewAbonentAgentFSMState}, Dst: ListeningMessagesAsHubAgentFSMState},
			{Name: SuspiciosHostInfoFromSegmentAbonentAgentFSMEvent, Src: []string{ListeningMessagesAsHubAgentFSMState}, Dst: BlockingSuspiciosHostAgentFSMState},
			{Name: HubDidNotBlockedSuspiciosHostAgentFSMEvent, Src: []string{BlockingSuspiciosHostAgentFSMState}, Dst: ListeningMessagesAsHubAgentFSMState},
			{Name: HubBlockedSuspiciosHostAgentFSMEvent, Src: []string{BlockingSuspiciosHostAgentFSMState}, Dst: AbonentsSegmentBroadcastAgentFSMState},
			{Name: HubPublishedInfoAgentFSMEvent, Src: []string{AbonentsSegmentBroadcastAgentFSMState}, Dst: ListeningMessagesAsHubAgentFSMState},
		},
		fsm.Callbacks{
			"enter_state": func(e_ context.Context, e *fsm.Event) {
				log.Printf("📦 FSM переход: %s -> %s по событию '%s'", e.Src, e.Dst, e.Event)
			},
		},
	)

	return a
}