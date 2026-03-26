package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"
)

func TestShareServerShareWritesLengthPrefixedPayload(t *testing.T) {
	resetTestLogChannel()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	server := &ShareServer{conns: map[net.Conn]bool{serverConn: true}}
	defer serverConn.Close()

	item := NewClipItem(TypeText, []byte("hello"))
	errCh := make(chan error, 1)
	go func() {
		server.Share(item)
		errCh <- nil
	}()

	header := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, header); err != nil {
		t.Fatalf("读取长度头失败: %v", err)
	}

	length := binary.BigEndian.Uint32(header)
	payload := make([]byte, length)
	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatalf("读取 payload 失败: %v", err)
	}

	var got ClipItem
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("反序列化 payload 失败: %v", err)
	}
	if string(got.Content) != "hello" {
		t.Fatalf("发送内容错误: %s", string(got.Content))
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Share 返回错误: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Share 未按预期完成")
	}
}

func TestShareServerRemovesBrokenConn(t *testing.T) {
	resetTestLogChannel()
	serverConn, clientConn := net.Pipe()
	clientConn.Close()

	server := &ShareServer{conns: map[net.Conn]bool{serverConn: true}}
	server.Share(NewClipItem(TypeText, []byte("hello")))

	if len(server.conns) != 0 {
		t.Fatalf("损坏连接应从连接池移除，实际剩余 %d", len(server.conns))
	}
}

func TestShareClientConnectToReceivesSharedItem(t *testing.T) {
	resetTestLogChannel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动测试 TCP 服务失败: %v", err)
	}
	defer ln.Close()

	item := NewClipItem(TypeText, []byte("from-server"))
	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		header := make([]byte, 4)
		binary.BigEndian.PutUint32(header, uint32(len(payload)))
		_, _ = conn.Write(header)
		_, _ = conn.Write(payload)
	}()

	client := NewShareClient(ln.Addr().String())
	received := make(chan *ClipItem, 1)
	closed := make(chan struct{}, 1)
	client.OnShared(func(item *ClipItem) {
		received <- item
	})
	client.OnClose(func() {
		closed <- struct{}{}
	})

	if !client.ConnectTo() {
		t.Fatalf("客户端应连接成功")
	}

	select {
	case got := <-received:
		if string(got.Content) != "from-server" {
			t.Fatalf("接收内容错误: %s", string(got.Content))
		}
		if got.From != FromRemote {
			t.Fatalf("收到的内容应标记为远端")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("客户端未收到共享内容")
	}

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatalf("连接关闭回调未触发")
	}

	<-serverDone
}
