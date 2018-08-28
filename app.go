package main

import (
	"cloudkarafka-mgmt/config"
	"cloudkarafka-mgmt/kafka"
	"cloudkarafka-mgmt/server"
	"cloudkarafka-mgmt/zookeeper"

	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var (
	port = flag.String("port", "8080", "Port to run HTTP server on")
	kh   = flag.String("kafka", "localhost:9092", "Hostname and port that the Kafka client should connect to")
	auth = flag.String("authentication", "scram", "Valid values are (none|none-with-write|scram)")
)

func main() {
	flag.Parse()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// Basic info
	zookeeper.Start()
	// Set authentication method for HTTP api
	fmt.Printf("[INFO] authentication-method=%s\n", *auth)
	zookeeper.SetAuthentication(*auth)
	// Runtime metrics, collect metrics every 30s
	// Consumer offsets
	go kafka.Start(*kh)
	// HTTP server
	config.Port = *port
	go server.Start()
	fmt.Println("CloudKarafka mgmt interface for Apache Kafka started")
	printMemUsage()
	//Wait for term
	for {
		select {
		case <-signals:
			time.AfterFunc(2*time.Second, func() {
				fmt.Println("[ERROR] could not exit in reasonable time")
				os.Exit(1)
			})
		case <-time.After(10 * time.Second):
			printMemUsage()
		}
	}
	fmt.Println("Stopping mgmt")
	kafka.Stop()
	zookeeper.Stop()
	fmt.Println("Stopped successfully")
	return
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc %v MiB\t", m.Alloc/1024/1024)
	fmt.Printf("TotalAlloc %v MiB\t", m.TotalAlloc/1024/1024)
	fmt.Printf("Sys %v MiB\t", m.Sys/1024/1024)
	fmt.Printf("NumGC %v\n", m.NumGC)
}
