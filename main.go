package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main(){
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
		fmt.Printf("You: %s\n", input)

	}
}