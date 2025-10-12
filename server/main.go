package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn  interface{}
	id   int64
	name string
	isWebSocket bool
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
        if client.isWebSocket {
            if wsConn, ok := client.conn.(*websocket.Conn); ok {
                wsConn.Close()
            }
        } else {
            if tcpConn, ok := client.conn.(net.Conn); ok {
                tcpConn.Close()
            }
        }
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
        var err error
        if client.isWebSocket {
            wsConn, ok := client.conn.(*websocket.Conn)
            if ok {
                err = wsConn.WriteMessage(websocket.TextMessage, message)
            } else {
                err = fmt.Errorf("invalid WebSocket connection type")
            }
        } else {
            tcpConn, ok := client.conn.(net.Conn)
            if ok {
                _, err = tcpConn.Write(message)
            } else {
                err = fmt.Errorf("invalid TCP connection type")
            }
        }

        if err != nil {
            cm.logger.Info("Failed to send to client",
                slog.Int64("client_id", client.id),
                slog.Bool("is_websocket", client.isWebSocket),
                slog.Any("error", err),
            )
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
        if client.isWebSocket {
            if wsConn, ok := client.conn.(*websocket.Conn); ok {
                wsConn.WriteMessage(websocket.TextMessage, []byte("Server is shutting down. Goodbye!"))
                wsConn.Close()
            }
        } else {
            if tcpConn, ok := client.conn.(net.Conn); ok {
                tcpConn.Write([]byte("Server is shutting down. Goodbye!\n"))
                tcpConn.Close()
            }
        }
    }
}

func handleConnection(cm *ClientManager, client *Client, wg *sync.WaitGroup, chatLog *os.File) {
	defer func() {
		cm.Remove(client.id)
		wg.Done() // -1 count
		cm.logger.Info("Client disconnected", slog.Int64("client_id", client.id))
	}()

	scanner := bufio.NewScanner(client.conn.(net.Conn))
	for scanner.Scan() {
		input := strings.Fields(scanner.Text())

		switch input[0] {
		case "/nick":
			if len(input) < 2 {
				if _, err := client.conn.(net.Conn).Write([]byte("Usage: /nick <new_name>\n")); err != nil {
					cm.logger.Info("Failed to send usage to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				}
				continue
			}
			newName := strings.TrimSpace(input[1])
			if newName == "" {
				newName = fmt.Sprintf("User%d", client.id)
				message := fmt.Sprintf("Name cannot be empty, so you will be %s\n", newName)
				client.conn.(net.Conn).Write([]byte(message))
			}
			cm.ChangeName(client.id, newName)
			if _, err := client.conn.(net.Conn).Write([]byte("Changed name successful\n")); err != nil {
				cm.logger.Info("Failed to send success message to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/users":
			list := cm.ListNames()
			if _, err := client.conn.(net.Conn).Write([]byte(list + "\n")); err != nil {
				cm.logger.Info("Failed to send /users to client", slog.Int64("client_id", client.id), slog.Any("error", err))
				continue
			}
		case "/msg":
			if len(input) < 3 {
				if _, err := client.conn.(net.Conn).Write([]byte("Usage: /msg <name> <text>\n")); err != nil {
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
					c.conn.(net.Conn).Write([]byte(msg))
					break
				}
			}
			cm.mutex.RUnlock()

			if !found {
				client.conn.(net.Conn).Write([]byte(fmt.Sprintf("User %s not found\n", targetName)))
			}
		default:
     text := scanner.Text()
    message := fmt.Sprintf("[Client %s]: %s\n", client.name, text)
    cm.Broadcast(client.id, []byte(message))

    logLine := fmt.Sprintf("[%s] [%s]: %s\n",
        time.Now().Format("2006-01-02 15:04:05"),
        client.name,
        text,
    )
    chatLog.WriteString(logLine)
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
        if client.isWebSocket {
            if wsConn, ok := client.conn.(*websocket.Conn); ok {
                wsConn.WriteMessage(websocket.TextMessage, msg)
            }
        } else {
            if tcpConn, ok := client.conn.(net.Conn); ok {
                tcpConn.Write(msg)
            }
        }
    }
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, cm *ClientManager, chatLog *os.File) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		cm.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	id := atomic.AddInt64(&clientIDcounter, 1)
	cm.logger.Info("WebSocket client connected", "client_id", id)

	_, msg, err := conn.ReadMessage()
    if err != nil {
        cm.logger.Info("WS client disconnected before sending name", "client_id", id)
        return
    }

    name := strings.TrimSpace(string(msg))
    if name == "" {
        name = fmt.Sprintf("User%d", id)

        conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Name cannot be empty, so you will be %s", name)))
    }
		client := &Client{
    conn:        conn,
    id:          id,
    name:        name,
    isWebSocket: true,
}
cm.Add(id, client)

    cm.logger.Info("WS client name set", "client_id", id, "name", name)

		for {
    messageType, msg, err := conn.ReadMessage()
    if err != nil {
        cm.logger.Info("WS client disconnected", "client_id", id, "error", err)
        cm.Remove(id)
        break
    }
    if messageType != websocket.TextMessage {
        continue // игнорируем бинарные сообщения
    }

    input := strings.Fields(string(msg))
    if len(input) == 0 {
        continue
    }

    switch input[0] {
    case "/nick":
        if len(input) < 2 {
            conn.WriteMessage(websocket.TextMessage, []byte("Usage: /nick <new_name>"))
            continue
        }
        newName := strings.TrimSpace(input[1])
        if newName == "" {
            newName = fmt.Sprintf("User%d", id)
            conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Name cannot be empty, so you will be %s", newName)))
        }
        cm.ChangeName(id, newName)
        conn.WriteMessage(websocket.TextMessage, []byte("Changed name successful"))

    case "/users":
        list := cm.ListNames()
        conn.WriteMessage(websocket.TextMessage, []byte(list))

    case "/msg":
        if len(input) < 3 {
            conn.WriteMessage(websocket.TextMessage, []byte("Usage: /msg <name> <text>"))
            continue
        }
        targetName := input[1]
        messageText := strings.Join(input[2:], " ")

        cm.mutex.RLock()
        var found bool
        for _, c := range cm.clients {
            if c.name == targetName {
                found = true
                privateMsg := fmt.Sprintf("[PM from %s]: %s", client.name, messageText)
                if c.isWebSocket {
                    if ws, ok := c.conn.(*websocket.Conn); ok {
                        ws.WriteMessage(websocket.TextMessage, []byte(privateMsg))
                    }
                } else {
                    if tcp, ok := c.conn.(net.Conn); ok {
                        tcp.Write([]byte(privateMsg + "\n"))
                    }
                }
                break
            }
        }
        cm.mutex.RUnlock()

        if !found {
            conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("User %s not found", targetName)))
        }

    default:
        text := string(msg)
        message := fmt.Sprintf("[Client %s]: %s", client.name, text)
        cm.Broadcast(client.id, []byte(message))

        logLine := fmt.Sprintf("[%s] [%s]: %s\n",
            time.Now().Format("2006-01-02 15:04:05"),
            client.name,
            text,
        )
        chatLog.WriteString(logLine)
    }
	}
}


var clientIDcounter int64
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer logFile.Close()

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	chatLog, err := os.OpenFile("chat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err != nil {
    log.Fatal(err)
}

	manager := NewClientManager(logger)
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error certification: %v\n", err)
		os.Exit(1)
	}
config := &tls.Config{Certificates: []tls.Certificate{cert}}
listener, err := tls.Listen("tcp", ":8080", config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server listening on :8080")

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "server/static/index.html")
})
http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
    handleWebSocket(w, r, manager, chatLog)
})
slog.Info("HTTP server starting on :8081")
    if err := http.ListenAndServe(":8081", nil); err != nil {
        slog.Error("HTTP server failed", "error", err)
    }
	}()

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

			client := &Client{
    conn:        conn,
    id:          id,
    name:        name,
    isWebSocket: false,
}
			manager.Add(id, client)

			wg.Add(1) //+1 count
			go handleConnection(manager, client, &wg, chatLog)
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
	chatLog.Close()

}
