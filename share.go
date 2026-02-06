package main

import (
	"bufio"
	"encoding/json"
	"fmt"
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
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	addrString := fmt.Sprintf("%v:%v",getLocalIP(),addr.Port)
	server := &ShareServer{
		ln: ln,
		addrString: addrString,
		conns: make(map[net.Conn]bool),
	}
	return server
}

func (s *ShareServer) Start() {
	go func ()  {
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				return
			}

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
				
				scanner := bufio.NewScanner(c)
				for scanner.Scan(){}
			}(conn)
		}
	}()
}

func (s *ShareServer) Stop(){
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
	data, err := json.Marshal(item)
	if(err != nil){
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns{
		conn.Write(data)
		conn.Write([]byte("\n"))
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
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return false
	}
	c.conn = conn
	go func(conn net.Conn) {
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var item ClipItem
			if err := json.Unmarshal(scanner.Bytes(), &item); err == nil {
				if c.onShare != nil{
					c.onShare(item.CloneToRemote())
				}
			}
		}

		if c.onClose != nil{
			c.onClose()
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
	if c.conn != nil{
		c.conn.Close()
	}
	c.conn = nil
}