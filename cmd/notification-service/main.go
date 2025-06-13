package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	log.Println("Notification Service starting...")
	
	for {
		fmt.Println("Notification Service is processing notifications...")
		time.Sleep(30 * time.Second)
	}
}