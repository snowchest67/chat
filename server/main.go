package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main(){
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
	go handleConnection(conn, clientID)
}

}

func handleConnection(conn net.Conn, id int){
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
    fmt.Printf("[Client %d]: %s\n", id, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
    fmt.Fprintf(os.Stderr, "[Client %d] error: %v\n", id, err)
	}
	fmt.Printf("[Client %d] disconnected\n", id)
}