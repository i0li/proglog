package discovery

import (
	"net"

	"go.uber.org/zap"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// メンバにはその状態を知るために以下のステータスを持つ
// Alive：サーバが存在し、健全
// Leaving：サーバーかクラスタからグレースフルに離脱中
// Left：サーバーがクラスタからグレースフルに離脱済み
// Failed：サーバーがクラスタから突然離脱
type Membership struct {
	Config
	handler Handler
	serf    *serf.Serf
	events  chan serf.Event
	logger  *zap.Logger
}

// handlerは今回はreplicator
func New(handler Handler, config Config) (*Membership, error) {
	c := &Membership{
		Config:  config,
		handler: handler,
		logger:  zap.L().Named("membership"),
	}
	if err := c.setupSerf(); err != nil {
		return nil, err
	}
	return c, nil
}

type Config struct {
	NodeName       string
	BindAddr       string
	Tags           map[string]string
	StartJoinAddrs []string
}

// ノードがクラスタに参加するための設定
func (m *Membership) setupSerf() (err error) {
	addr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
	if err != nil {
		return err
	}
	config := serf.DefaultConfig()
	config.Init()
	// ゴシッププロトコルでこのノードと通信する際のアドレスとポートを登録
	config.MemberlistConfig.BindAddr = addr.IP.String()
	config.MemberlistConfig.BindPort = addr.Port
	// EventChはノードがクラスタに参加・離脱した際にSerfのイベントを受け取る手段
	// Serfのイベントをm.eventsに送信するように設定
	m.events = make(chan serf.Event)
	config.EventCh = m.events
	// Tagsはカスタムのメターデータを付与する役割
	// 今回はこのノードのRPCを呼び出す際のアドレスの情報を格納する
	config.Tags = m.Tags
	// クラスタ全体で一意なノードの識別子
	config.NodeName = m.Config.NodeName
	// SerfがJoinイベントを発火
	m.serf, err = serf.Create(config)
	if err != nil {
		return err
	}
	go m.eventHandler()
	// StartJoinAddrsには初めに接続する既存ノードのアドレスが格納されている
	// DNSのデフォルトルートのようなもの？
	// 本番環境ではノードの障害やネットワークの中断に対応できるように少なくとも3つのアドレスを指定
	if m.StartJoinAddrs != nil {
		_, err = m.serf.Join(m.StartJoinAddrs, true)
		if err != nil {
			return err
		}
	}
	return nil
}

type Handler interface {
	Join(name, addr string) error
	Leave(name string) error
}

func (m *Membership) eventHandler() {
	for e := range m.events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			// Serfは複数のメンバーの更新を1つのイベントにまとめることがある
			// そのためイベントのMembersでループしている
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					continue
				}
				m.handleJoin(member)
			}
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					return
				}
				m.handleLeave(member)
			}
		}
	}
}

func (m *Membership) handleJoin(member serf.Member) {
	if err := m.handler.Join(
		member.Name,
		member.Tags["rpc_addr"],
	); err != nil {
		m.logError(err, "failed to join", member)
	}
}

func (m *Membership) handleLeave(member serf.Member) {
	if err := m.handler.Leave(
		member.Name,
	); err != nil {
		m.logError(err, "failed to leave", member)
	}
}

// 指定されたSerfメンバーが呼び出し元のノード自身であるかどうかをチェック
func (m *Membership) isLocal(member serf.Member) bool {
	return m.serf.LocalMember().Name == member.Name
}

func (m *Membership) Members() []serf.Member {
	return m.serf.Members()
}

func (m *Membership) Leave() error {
	return m.serf.Leave()
}

func (m *Membership) logError(err error, msg string, member serf.Member) {
	log := m.logger.Error
	if err == raft.ErrNotLeader {
		log = m.logger.Debug
	}
	log(
		msg,
		zap.Error(err),
		zap.String("name", member.Name),
		zap.String("rpc_addr", member.Tags["rpc_addr"]),
	)
}
