package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Connected to server.")
	fmt.Println("Enter message (or /quit to exit):")

	go getMessage(conn)

	reader := bufio.NewReader(os.Stdin)
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
			break
		}
	}
}

func getMessage(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Print(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Connection closed: %v\n", err)
		return
	}
}
