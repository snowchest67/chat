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

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("Ошибка при принятии подключения: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Client connected")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
    fmt.Printf("Message: %s\n", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
    fmt.Printf("Error reading: %v\n", err)
	}
	fmt.Println("Client disconnected")

}