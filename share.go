package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
)

type ShareServer struct{
	ln net.Listener
	addrString string
	conns map[net.Conn]bool
	mu sync.Mutex
}

func getLocalIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return "127.0.0.1"
    }
    defer conn.Close()
    return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func NewShareServer() * ShareServer{
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "tcp服务器正在启动..."}
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	addrString := fmt.Sprintf("%v:%v",getLocalIP(),addr.Port)
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("tcp服务器已启动，地址为%s", addrString)}
	server := &ShareServer{
		ln: ln,
		addrString: addrString,
		conns: make(map[net.Conn]bool),
	}
	return server
}

func (s *ShareServer) Start() {
	go func ()  {
		global_log_channel <- LogEntry{Kind: KindInfo, Content: "tcp服务器正在监听连接..."}
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				return
			}

			global_log_channel <- LogEntry{Kind: KindInfo, Content: "一个客户端已连接"}
			s.mu.Lock()
			s.conns[conn] = true
			s.mu.Unlock()

			go func (c net.Conn)  {
			    defer func() {
					c.Close()
					s.mu.Lock()
					delete(s.conns, c)
					s.mu.Unlock()
				}()
				
				// 保持连接，不读取数据
				buf := make([]byte, 1)
				for {
					_, err := c.Read(buf)
					if err != nil {
						break
					}
				}
			}(conn)
		}
	}()
}

func (s *ShareServer) Stop(){
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "tcp服务器正在关闭..."}
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.conns {
		conn.Close()
	}
	s.ln.Close()
}

func (s *ShareServer) AddrString() string {
	return s.addrString
}

func (s *ShareServer) Share(item *ClipItem) {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "准备发送剪贴板内容。"}
	data, err := json.Marshal(item)
	if(err != nil){
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 使用长度前缀协议：4字节长度 + JSON数据
	length := uint32(len(data))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("发送剪贴板内容，长度为%d字节", length)}
	for conn := range s.conns{
		conn.Write(header)
		conn.Write(data)
	}
}


type ShareClient struct{
	addr string
	conn net.Conn
	onShare func(item *ClipItem)
	onClose func()
}

func NewShareClient(addr string) *ShareClient{
	return &ShareClient{
		addr: addr,
		conn: nil,
	}
}

func (c *ShareClient) ConnectTo() bool{
	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("正在连接到服务器%s...", c.addr)}
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return false
	}
	c.conn = conn
	go func(conn net.Conn) {
		defer func ()  {
			conn.Close()

			if c.onClose != nil{
				c.onClose()
			}
		}()

		// 使用长度前缀协议读取
		header := make([]byte, 4)
		for {
			// 读取4字节长度头
			if _, err := io.ReadFull(conn, header); err != nil {
				break
			}
			global_log_channel <- LogEntry{Kind: KindInfo, Content: "收到剪贴板内容，正在读取..."}
			
			length := binary.BigEndian.Uint32(header)
			
			// 读取完整的JSON数据
			data := make([]byte, length)
			if _, err := io.ReadFull(conn, data); err != nil {
				break
			}
			
			var item ClipItem
			if err := json.Unmarshal(data, &item); err == nil {
				if c.onShare != nil{
					c.onShare(item.CloneToRemote())
				}
			}
		}
	}(c.conn)
	return true;
}

func (c *ShareClient) OnShared(callback func(item *ClipItem)){
	c.onShare = callback
}

func (c *ShareClient) OnClose(callback func()){
	c.onClose = callback
}

func (c *ShareClient) Close() {
	global_log_channel <- LogEntry{Kind: KindInfo, Content: "正在关闭与服务器的连接..."}
	if c.conn != nil{
		c.conn.Close()
	}
	c.conn = nil
}