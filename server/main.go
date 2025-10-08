package main

import (
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	conn net.Conn
	id   int64
	name string
}

type ClientManager struct {
	clients map[int64]*Client
	mutex   sync.RWMutex
	logger  *slog.Logger
}

func NewClientManager(logger *slog.Logger) *ClientManager {
	return &ClientManager{
		clients: make(map[int64]*Client),
		logger:  logger,
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

	flag := false
	for _, client := range clients {
		if _, err := client.conn.Write(message); err != nil {
			cm.logger.Info("Failed to send to client",
				slog.Int64("client_id", client.id),
				slog.Any("error", err),
			)
		} else if !flag {
			cm.logger.Info(string(message))
			flag = true
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

func (cm *ClientManager) CloseAll() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	for _, client := range cm.clients {
		client.conn.Close()
	}
}

func handleConnection(cm *ClientManager, client *Client, wg *sync.WaitGroup) {
	defer func() {
		cm.Remove(client.id)
		wg.Done() // -1 count
		cm.logger.Info("Client disconnected", slog.Int64("client_id", client.id))
	}()

	scanner := bufio.NewScanner(client.conn)
	for scanner.Scan() {
		input := strings.Fields(scanner.Text())

		switch input[0] {
		case "/nick":
			if len(input) < 2 {
				if _, err := client.conn.Write([]byte("Usage: /nick <new_name>\n")); err != nil {
					cm.logger.Info("Failed to send usage to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				}
				continue
			}
			newName := strings.TrimSpace(input[1])
			if newName == "" {
				newName = fmt.Sprintf("User%d", client.id)
				message := fmt.Sprintf("Name cannot be empty, so you will be %s\n", newName)
				client.conn.Write([]byte(message))
			}
			cm.ChangeName(client.id, newName)
			if _, err := client.conn.Write([]byte("Changed name successful\n")); err != nil {
				cm.logger.Info("Failed to send success message to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/users":
			list := cm.ListNames()
			if _, err := client.conn.Write([]byte(list + "\n")); err != nil {
				cm.logger.Info("Failed to send /users to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/msg":
			if len(input) < 3 {
				if _, err := client.conn.Write([]byte("Usage: /msg <name> <text>\n")); err != nil {
					cm.logger.Info("Failed to send usage to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				}
				continue
			}
			targetName := input[1]
			messageText := strings.Join(input[2:], " ")

			cm.mutex.RLock()
			var found bool
			for _, c := range cm.clients {
				if c.name == targetName {
					found = true
					msg := fmt.Sprintf("[PM from %s]: %s\n", client.name, messageText)
					c.conn.Write([]byte(msg))
					break
				}
			}
			cm.mutex.RUnlock()

			if !found {
				client.conn.Write([]byte(fmt.Sprintf("User %s not found\n", targetName)))
			}
		default:
			message := fmt.Sprintf("[Client %s]: %s\n", client.name, scanner.Text())
			cm.Broadcast(client.id, []byte(message))
		}

	}

	if err := scanner.Err(); err != nil {
		cm.logger.Info("Client scan error", slog.Int64("client.id", client.id), slog.Any("err", err))
	}
}

func (cm *ClientManager) NotifyShutdown() {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	msg := []byte("Server is shutting down. Goodbye!\n")
	for _, client := range cm.clients {
		client.conn.Write(msg)
	}
}

var clientIDcounter int64

func main() {
	logFile, err := os.OpenFile("chat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer logFile.Close()

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	manager := NewClientManager(logger)
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server listening on :8080")

	sigChan := make(chan os.Signal, 1)   //chan for interrupt
	signal.Notify(sigChan, os.Interrupt) // os.Interrupt = Ctrl+C
	var wg sync.WaitGroup

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				slog.Info("Accept failed, stopping accepting new connections", slog.Any("error", err))
				return
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

			wg.Add(1) //+1 count
			go handleConnection(manager, client, &wg)
		}
	}()

	<-sigChan
	slog.Info("Shutting down...")
	listener.Close()
	manager.NotifyShutdown()
	manager.CloseAll()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All clients disconnected")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout! Forcing shutdown")
	}

}
