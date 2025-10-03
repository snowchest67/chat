package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

func main(){
	listener, err := net.Listen("tcp",":8080") //создаём слушателя, который «слушает» входящие соединения на определённом адресе и порту
	if err != nil{
		fmt.Printf("Произошла ошибка при прослушивании: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Сервер запущен на :8080, ожидание подключения...")

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("Ошибка при принятии подключения: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for{
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Клиент закрыл соединение.")
				break
			}
			fmt.Printf("Ошибка при чтении: %v\n", err)
			break
		}
		fmt.Printf("Получено от клиента: %s", buffer[:n])
	}


}