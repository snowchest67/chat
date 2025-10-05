package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

type Client struct {
	conn net.Conn
	id int
}

type ClientMap map[int]*Client

func main(){
	var clients ClientMap = make(ClientMap)
	var mutex sync.Mutex
	listener, err := net.Listen("tcp",":8080") //создаём слушателя, который «слушает» входящие соединения на определённом адресе и порту
	if err != nil {
		fmt.Printf("Произошла ошибка при прослушивании: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server listening on :8080")

count := 0
for{
	conn, err := listener.Accept()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		continue
	}
	count++
	clientID := count
	fmt.Printf("[Client %d] connected\n", clientID)
	go handleConnection(conn, clientID, &clients, &mutex)
}

}

func handleConnection(conn net.Conn, id int, clients *ClientMap, mutex *sync.Mutex){
	defer conn.Close()
	mutex.Lock()
	(*clients)[id] = &Client{conn, id}
	mutex.Unlock()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
    input := scanner.Text()
    for _, n := range *clients {
        if n.id == id {
            continue
        }
        message := fmt.Sprintf("[Client %d] %s", id, input)

        _, err := n.conn.Write([]byte(message))
        if err != nil {
            fmt.Fprintf(os.Stderr, "[Client %d] error: %v\n", id, err)
            continue
        }
    }
}

	if err := scanner.Err(); err != nil {
    fmt.Fprintf(os.Stderr, "[Client %d] error: %v\n", id, err)
	}
	fmt.Printf("[Client %d] disconnected\n", id)
	mutex.Lock()
	delete(*clients, id)
	mutex.Unlock()
	
}