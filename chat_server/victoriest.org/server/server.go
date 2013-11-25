package server

import (
	"../probe"
	"../protocol"
	"../utils"
	"bufio"
	log "code.google.com/p/log4go"
	"net"
)

type IVictoriestServer interface {
	Startup()
	Shutdown()
}

type ServerHandler func(*VictoriestServer, interface{})

type VictoriestServer struct {
	// 服务端端口号
	port string
	// 退出信号量
	quitSp chan bool
	// 客户端连接Map
	connMap map[string]*net.TCPConn
	//
	Handler ServerHandler
}

func NewVictoriestServer(port string) *VictoriestServer {
	server := new(VictoriestServer)
	server.port = port
	// tcpConn的map
	server.connMap = make(map[string]*net.TCPConn)
	// 退出信号channel
	server.quitSp = make(chan bool)
	return server
}

/**
 * 客户端连接管理器
 */
func (self *VictoriestServer) initConnectionManager(tcpListener *net.TCPListener) {
	i := 0
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			log.Error(err.Error())
			continue
		}

		self.connMap[tcpConn.RemoteAddr().String()] = tcpConn
		i++

		log.Debug("A client connected : ", tcpConn.RemoteAddr().String())
		go self.tcpHandler(*tcpConn)
	}
}

/**
 * 开启服务器
 */
func (self *VictoriestServer) Startup() {
	strAddr := ":" + self.port

	// 构造tcpAddress
	tcpAddr, err := net.ResolveTCPAddr("tcp", strAddr)
	utils.CheckError(err, true)

	// 创建TCP监听
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	utils.CheckError(err, true)
	defer tcpListener.Close()
	log.Debug("Start listen ", tcpListener.Addr().String())

	// 连接管理
	self.initConnectionManager(tcpListener)
}

/**
 * 关闭服务器指令
 */
func (self *VictoriestServer) Shutdown() {
	// 关闭所有连接
	for _, conn := range self.connMap {
		log.Debug("close:" + conn.RemoteAddr().String())
		conn.Close()
	}
	log.Debug("Shutdown")
}

/**
 * 一客户端一线程
 */
func (self *VictoriestServer) tcpHandler(tcpConn net.TCPConn) {
	ipStr := tcpConn.RemoteAddr().String()
	defer func() {
		log.Debug("disconnected :" + ipStr)
		self.BroadcastMessage("disconnected :" + ipStr)
		tcpConn.Close()
		delete(self.connMap, ipStr)
	}()
	self.BroadcastMessage("A new connection :" + ipStr)
	reader := bufio.NewReader(&tcpConn)
	for {
		jsonProbe := new(probe.JsonProbe)
		message, _, err := jsonProbe.DeserializeByReader(reader)
		if err != nil {
			return
		}
		// use pack do what you want ...
		self.Handler(self, message)
	}
}

func (self *VictoriestServer) BroadcastMessage(message interface{}) {
	jsonProbe := new(probe.JsonProbe)
	buff, _ := jsonProbe.Serialize(message, protocol.MSG_TYPE_TEST_MESSGAE)
	// 向所有人发话
	for _, conn := range self.connMap {
		conn.Write(buff)
	}
}
