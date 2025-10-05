package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main(){
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("Не удалось подключиться к серверу: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Connected to server.\nEnter message (or /quit to exit):")
	reader := bufio.NewReader(os.Stdin)
	buff := make([]byte, 1024)
	go getMessage(conn, buff)

	for {
		input, err := reader.ReadString('\n') 
		if err != nil {
			fmt.Printf("Произошла ошибка при чтении строки %s\n", err)
			os.Exit(1)
		}
		input = strings.TrimSpace(input)
		if input == "/quit" {
			fmt.Println("Disconnected.")
			break
		}
		_, err = fmt.Fprintln(conn, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending: %v\n", err)
			break
		}

	}
}

func getMessage(conn net.Conn, buff []byte) {
	for {
		n, err := conn.Read(buff)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
    fmt.Print(string(buff[0:n]))
    fmt.Println()
	}
}