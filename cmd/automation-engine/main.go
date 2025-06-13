package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	log.Println("Automation Engine starting...")
	
	for {
		fmt.Println("Automation Engine is processing jobs...")
		time.Sleep(30 * time.Second)
	}
}