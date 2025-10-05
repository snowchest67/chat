package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

type Client struct {
	conn net.Conn
	id   int64
}

type ClientManager struct {
	clients map[int64]*Client
	mutex   sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[int64]*Client),
	}
}

func (cm *ClientManager) Add(id int64, client *Client) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.clients[id] = client
}

func (cm *ClientManager) Remove(id int64) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	if client, ok := cm.clients[id]; ok {
		client.conn.Close()
		delete(cm.clients, id)
	}
}

func (cm *ClientManager) Broadcast(exceptID int64, message []byte) {
	cm.mutex.RLock()
	clients := make([]*Client, 0, len(cm.clients))
	for id, client := range cm.clients {
		if id != exceptID {
			clients = append(clients, client)
		}
	}
	cm.mutex.RUnlock()

	for _, client := range clients {
		if _, err := client.conn.Write(message); err != nil {
			slog.Info("Failed to send to client",
				slog.Int64("client_id", client.id),
				slog.Any("error", err),
			)
			go cm.Remove(client.id)
		}
	}
}

var clientIDcounter int64

func main() {
	manager := NewClientManager()
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server listening on :8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Accept error: %v\n", err)
			continue
		}

		id := atomic.AddInt64(&clientIDcounter, 1)
		slog.Info("Client connected", slog.Int64("client_id", id))

		client := &Client{conn: conn, id: id}
		manager.Add(id, client)

		go handleConnection(manager, client)

	}
}

func handleConnection(manager *ClientManager, client *Client) {
	defer func() {
		manager.Remove(client.id)
		slog.Info("Client disconnected", slog.Int64("client_id", client.id))
	}()

	scanner := bufio.NewScanner(client.conn)
	for scanner.Scan() {
		message := fmt.Sprintf("[Client %d]: %s\n", client.id, scanner.Text())
		manager.Broadcast(client.id, []byte(message))
	}

	if err := scanner.Err(); err != nil {
		slog.Info("Client scan error", slog.Int64("client.id", client.id), slog.Any("err", err))
	}
}
