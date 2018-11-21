package server

import (
	"bufio"
	"net"
	"net/http"

	"../codec"
	"../dao"
	"../protocol"
	"../utils"
	log "github.com/alecthomas/log4go"
	"github.com/gorilla/websocket"
)

// INexus 服务器接口
type INexus interface {
	// 启动服务器
	Startup()
	// 关闭服务器
	Shutdown()
}

// MessageRecivedHandler 消息处理托管
type MessageRecivedHandler func(*Nexus, string, *protocol.MobileSuiteModel)

// ConnectionHandler 连接状态处理托管
type ConnectionHandler func(*Nexus, string)

// ProtocolType 服务器协议类型
type ProtocolType int

const (
	// ProtocolTypeTCP tcp
	ProtocolTypeTCP ProtocolType = iota
	// ProtocolTypeWebSocket ws
	ProtocolTypeWebSocket
)

// Nexus the net connector struct
type Nexus struct {
	protocolType         ProtocolType           // 连接类型(tcp, ws)
	port                 string                 // 服务端端口号
	quitSemaphore        chan bool              // 退出信号量
	connMap              map[string]interface{} // 客户端连接Map
	recivedHandler       MessageRecivedHandler  // 消息逻辑处理托管Handler
	newConnectionHandler ConnectionHandler      // 新连接处理Handler
	disconnectHandler    ConnectionHandler      // 断开连接处理Handler
	probe                codec.ProtobufProbe    // 序列化接口
	DbConnector          *dao.MysqlConnector    // 数据库连接器
	wsUpgrader           *websocket.Upgrader    // ws服务的管理对象
}

// NewNexus create a net connector
func NewNexus(protocolType ProtocolType,
	port string, handler MessageRecivedHandler,
	connHander ConnectionHandler, disconnHander ConnectionHandler,
	dbCon *dao.MysqlConnector) *Nexus {
	nexus := new(Nexus)
	nexus.protocolType = protocolType
	nexus.port = port
	nexus.connMap = make(map[string]interface{})
	nexus.quitSemaphore = make(chan bool)
	nexus.recivedHandler = handler
	nexus.newConnectionHandler = connHander
	nexus.disconnectHandler = disconnHander
	nexus.probe = *new(codec.ProtobufProbe)
	nexus.DbConnector = dbCon
	nexus.wsUpgrader = nil
	return nexus
}

// initTCPConnectionManager 初始化客户端连接管理器TCP
func (nexus *Nexus) initTCPConnectionManager(tcpListener *net.TCPListener) {
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			log.Error(err.Error())
			continue
		}

		nexus.connMap[tcpConn.RemoteAddr().String()] = tcpConn

		log.Info("A client connected : ", tcpConn.RemoteAddr().String())
		go nexus.tcpPipe(tcpConn)
	}
}

// initWsConnectionManager 初始化客户端连接管理器WebSocket
func (nexus *Nexus) initWsConnectionManager(w http.ResponseWriter, r *http.Request) {
	nexus.wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }
	wsConn, err := nexus.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err.Error())
		return
	}

	nexus.connMap[wsConn.RemoteAddr().String()] = wsConn
	log.Info("A client connected : ", wsConn.RemoteAddr().String())
	go nexus.wsPipe(wsConn)
}

// Startup 启动服务器
func (nexus *Nexus) Startup() {
	strAddr := ":" + nexus.port
	if nexus.protocolType == ProtocolTypeTCP {
		tcpAddr, err := net.ResolveTCPAddr("tcp", strAddr)
		utils.CheckError(err, true)

		tcpListener, err := net.ListenTCP("tcp", tcpAddr)
		utils.CheckError(err, true)

		defer tcpListener.Close()
		log.Info("Start listen ", tcpListener.Addr().String())

		// TCP连接管理
		nexus.initTCPConnectionManager(tcpListener)
	} else if nexus.protocolType == ProtocolTypeWebSocket {
		// WebSocket连接管理
		nexus.wsUpgrader = &websocket.Upgrader{}
		http.HandleFunc("/ws", nexus.initWsConnectionManager)
		err := http.ListenAndServe(strAddr, nil)
		utils.CheckError(err, true)
	}
}

// Shutdown 关闭服务器指令
func (nexus *Nexus) Shutdown() {
	// 关闭所有连接
	for _, conn := range nexus.connMap {
		con := conn.(net.Conn)
		con.Close()
		log.Info("close:" + con.RemoteAddr().String())
		con.Close()
	}
	log.Info("Shutdown")
}

// tcpPipe 分派客户端长连接协程 一客户端一协程
func (nexus *Nexus) tcpPipe(tcpConn net.Conn) {
	ipStr := tcpConn.RemoteAddr().String()
	defer func() {
		log.Info("disconnected :" + ipStr)
		nexus.disconnectHandler(nexus, ipStr)

		tcpConn.Close()
		delete(nexus.connMap, ipStr)
	}()
	nexus.newConnectionHandler(nexus, ipStr)

	reader := bufio.NewReader(tcpConn)

	for {
		message, _, err := nexus.probe.DeserializeByReader(reader)
		if err != nil {
			return
		}
		nexus.recivedHandler(nexus, ipStr, message)
	}
}

// wsPipe 分派客户端长连接协程 一客户端一协程
func (nexus *Nexus) wsPipe(wsConn *websocket.Conn) {
	ipStr := wsConn.RemoteAddr().String()
	defer func() {
		log.Info("disconnected :" + ipStr)
		nexus.disconnectHandler(nexus, ipStr)

		wsConn.Close()
		delete(nexus.connMap, ipStr)
	}()
	nexus.newConnectionHandler(nexus, ipStr)

	for {
		_, reader, err := wsConn.NextReader()
		br := bufio.NewReader(reader)
		message, _, err := nexus.probe.DeserializeByReader(br)
		if err != nil {
			return
		}
		nexus.recivedHandler(nexus, ipStr, message)
	}
}

// BroadcastMessage 全局广播
func (nexus *Nexus) BroadcastMessage(message *protocol.MobileSuiteModel) {
	buff, _ := nexus.probe.Serialize(message)
	// 向所有人发话
	for _, conn := range nexus.connMap {
		switch nexus.protocolType {
		case ProtocolTypeTCP:
			conn.(net.Conn).Write(buff)
		case ProtocolTypeWebSocket:
			// TODO message type
			conn.(*websocket.Conn).WriteMessage(1, buff)
		}

	}
}

// SendTo 向指定ip发消息
func (nexus *Nexus) SendTo(sendTo string, message *protocol.MobileSuiteModel) {
	buff, _ := nexus.probe.Serialize(message)
	switch nexus.protocolType {
	case ProtocolTypeTCP:
		nexus.connMap[sendTo].(net.Conn).Write(buff)
	case ProtocolTypeWebSocket:
		// TODO message type
		nexus.connMap[sendTo].(*websocket.Conn).WriteMessage(1, buff)
	}
}

// ConnectionIsOpen 指定ip是否建立连接
func (nexus *Nexus) ConnectionIsOpen(ipStr string) bool {
	conn := nexus.connMap[ipStr]
	if conn != nil {
		return true
	}
	return false
}
