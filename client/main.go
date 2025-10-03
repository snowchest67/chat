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
	}
	defer conn.Close()
	fmt.Println("Enter message (or /quit to exit):")
	reader := bufio.NewReader(os.Stdin)

	for {
		input, err := reader.ReadString('\n') 
		if err != nil {
			fmt.Printf("Произошла ошибка при чтении строки %s\n", err)
			os.Exit(1)
		}
		input = strings.TrimSpace(input)
		if input == "/quit" {
			fmt.Println("Goodbye!")
			break
		}
		_, err = conn.Write([]byte(input+"\n"))
		if err != nil {
			fmt.Printf("Ошибка при отправке: %v\n", err)
			break
		}

	}
}