package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	conn, err := tls.Dial("tcp", "localhost:8080", &tls.Config{
    InsecureSkipVerify: true, // для самоподписанных сертификатов
})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Connected to server.")
	fmt.Print("Enter your name: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	_, err = fmt.Fprintln(conn, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Send error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Enter message (or /quit to exit):")

	go getMessage(conn)

loop:
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)
		if input == "/quit" {
			fmt.Println("Disconnected.")
			break
		}
		_, err = fmt.Fprintln(conn, input)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Send error: %v\n", err)
        break loop
    }

	}
}

func getMessage(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Connection closed: %v\n", err)
		return
	}
}
