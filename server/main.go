package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

type Client struct {
	conn net.Conn
	id   int64
	name string
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

func (cm *ClientManager) ChangeName(id int64, newName string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.clients[id].name = newName
}

func (cm *ClientManager) ListNames() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var names []string
	for _, client := range cm.clients {
		if client.name != "" {
			names = append(names, client.name)
		}
	}

	return "Online: " + strings.Join(names, ", ")
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

		nameScanner := bufio.NewScanner(conn)
		if !nameScanner.Scan() {
			conn.Close()
			slog.Info("Client disconnected before sending name", slog.Int64("client_id", id))
			continue
		}
		name := strings.TrimSpace(nameScanner.Text())
		if name == "" {
			name = fmt.Sprintf("User%d", id)
			conn.Write([]byte(fmt.Sprintf("Name cannot be empty, so you will be %s\n", name)))
		}

		client := &Client{conn: conn, id: id, name: name}
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
		input := strings.Fields(scanner.Text())

		switch input[0] {
		case "/nick":
			if len(input) < 2 {
				if _, err := client.conn.Write([]byte("Usage: /nick <new_name>\n")); err != nil {
					slog.Info("Failed to send usage to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				}
				continue
			}
			newName := strings.TrimSpace(input[1])
			if newName == "" {
				newName = fmt.Sprintf("User%d", client.id)
				message := fmt.Sprintf("Name cannot be empty, so you will be %s\n", newName)
				client.conn.Write([]byte(message))
			}
			manager.ChangeName(client.id, newName)
			if _, err := client.conn.Write([]byte("Changed name successful\n")); err != nil {
				slog.Info("Failed to send success message to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/users":
			list := manager.ListNames()
			if _, err := client.conn.Write([]byte(list + "\n")); err != nil {
				slog.Info("Failed to send /users to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/msg":
			if len(input) < 3 {
				if _, err := client.conn.Write([]byte("Usage: /msg <name> <text>\n")); err != nil {
					slog.Info("Failed to send usage to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				}
				continue
			}
			targetName := input[1]
			messageText := strings.Join(input[2:], " ")

			manager.mutex.RLock()
			var found bool
			for _, c := range manager.clients {
				if c.name == targetName {
					found = true
					msg := fmt.Sprintf("[PM from %s]: %s\n", client.name, messageText)
					c.conn.Write([]byte(msg))
					break
				}
			}
			manager.mutex.RUnlock()

			if !found {
				client.conn.Write([]byte(fmt.Sprintf("User %s not found\n", targetName)))
			}
		default:
			message := fmt.Sprintf("[Client %s]: %s\n", client.name, scanner.Text())
			manager.Broadcast(client.id, []byte(message))
		}

	}

	if err := scanner.Err(); err != nil {
		slog.Info("Client scan error", slog.Int64("client.id", client.id), slog.Any("err", err))
	}
}
