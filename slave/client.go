package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"distributed-db/shared"
	"log"
)

const (
	masterHost        = "localhost"
	masterPort        = "8083"
	validToken        = "secret-token"
	heartbeatInterval = 30 * time.Second
)

var (
	masterConn net.Conn
	connMutex  sync.Mutex
)

func isMasterQuery(query string) bool {
	query = strings.ToUpper(strings.TrimSpace(query))
	return strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "DROP")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting interface addresses: %v", err)
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ip := ipnet.IP.String()
			log.Printf("Found local IP address: %s", ip)
			return ip
		}
	}
	log.Printf("No suitable IP address found, using hostname")
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Error getting hostname: %v", err)
		return "unknown"
	}
	return hostname
}

func establishMasterConnection() error {
	connMutex.Lock()
	defer connMutex.Unlock()

	if masterConn != nil {
		req := shared.DBRequest{
			Query:     "SELECT 1",
			Token:     validToken,
			FromSlave: getLocalIP(),
			IsSelect:  true,
		}
		reqData, err := json.Marshal(req)
		if err == nil {
			reqData = append(reqData, '\n')
			if _, err := masterConn.Write(reqData); err == nil {
				scanner := bufio.NewScanner(masterConn)
				if scanner.Scan() {
					var resp shared.DBResponse
					if err := json.Unmarshal([]byte(scanner.Text()), &resp); err == nil && resp.Status == "ok" {
						return nil
					}
				}
			}
		}
		if masterConn != nil {
			masterConn.Close()
			masterConn = nil
		}
	}

	var err error
	masterConn, err = net.Dial("tcp", masterHost+":"+masterPort)
	if err != nil {
		return fmt.Errorf("failed to connect to master server: %v", err)
	}

	if tcpConn, ok := masterConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true)
	}

	req := shared.DBRequest{
		Query:     "SELECT 1",
		Token:     validToken,
		FromSlave: getLocalIP(),
		IsSelect:  true,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("failed to marshal initial request: %v", err)
	}
	reqData = append(reqData, '\n')
	if _, err := masterConn.Write(reqData); err != nil {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("failed to send initial request: %v", err)
	}

	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for range ticker.C {
			if err := sendHeartbeat(); err != nil {
				log.Printf("Heartbeat failed: %v", err)
				if err := establishMasterConnection(); err != nil {
					log.Printf("Failed to reconnect to master: %v", err)
				}
			}
		}
	}()

	log.Printf("Established persistent connection to master server from %s", getLocalIP())
	return nil
}

func sendHeartbeat() error {
	connMutex.Lock()
	defer connMutex.Unlock()

	if masterConn == nil {
		return fmt.Errorf("no connection to master")
	}

	req := shared.DBRequest{
		Query:     "SELECT 1",
		Token:     validToken,
		FromSlave: getLocalIP(),
		IsSelect:  true,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat request: %v", err)
	}

	reqData = append(reqData, '\n')
	if _, err := masterConn.Write(reqData); err != nil {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("failed to send heartbeat: %v", err)
	}

	scanner := bufio.NewScanner(masterConn)
	if !scanner.Scan() {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("failed to read heartbeat response: %v", scanner.Err())
	}

	response := scanner.Text()
	var resp shared.DBResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("invalid heartbeat response: %v", err)
	}

	if resp.Status != "ok" {
		masterConn.Close()
		masterConn = nil
		return fmt.Errorf("heartbeat failed: %s", resp.Message)
	}

	log.Printf("Heartbeat successful from slave %s", getLocalIP())
	return nil
}

func sendQueryToMaster(query string) (shared.DBResponse, error) {
	if isMasterQuery(query) {
		return shared.DBResponse{
			Status:  "error",
			Message: "This query can only be executed on the master server. Please use the master server interface.",
		}, nil
	}

	if err := establishMasterConnection(); err != nil {
		return shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to connect to master server: %v", err),
		}, err
	}

	connMutex.Lock()
	defer connMutex.Unlock()

	if masterConn == nil {
		return shared.DBResponse{
			Status:  "error",
			Message: "No connection to master server",
		}, fmt.Errorf("no connection to master")
	}

	req := shared.DBRequest{
		Query:     query,
		Token:     validToken,
		FromSlave: getLocalIP(),
		IsSelect:  strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT"),
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to marshal request: %v", err),
		}, err
	}

	log.Printf("Sending request to master: %s", string(reqData))
	reqData = append(reqData, '\n')
	if _, err := masterConn.Write(reqData); err != nil {
		masterConn.Close()
		masterConn = nil
		return shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to send request: %v", err),
		}, err
	}

	scanner := bufio.NewScanner(masterConn)
	if !scanner.Scan() {
		masterConn.Close()
		masterConn = nil
		return shared.DBResponse{
			Status:  "error",
			Message: "Failed to read response from master server",
		}, scanner.Err()
	}

	response := scanner.Text()
	log.Printf("Received response from master: %s", response)

	var resp shared.DBResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid response from master server: %v", err),
		}, nil
	}

	return resp, nil
}

func replicateData(req *shared.ReplicationRequest) error {
	conn, err := net.Dial("tcp", masterHost+":"+masterPort)
	if err != nil {
		return fmt.Errorf("failed to connect to master server: %v", err)
	}
	defer conn.Close()

	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	reqData = append(reqData, '\n')
	if _, err := conn.Write(reqData); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return fmt.Errorf("failed to read response from master server: %v", scanner.Err())
	}

	response := scanner.Text()
	var resp shared.ReplicationResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return fmt.Errorf("invalid response from master server: %v", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("replication failed: %s", resp.Message)
	}

	return nil
}

func handleAPIQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	query := strings.ToUpper(strings.TrimSpace(req.Query))
	if strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "DROP") {
		http.Error(w, "CREATE and DROP operations are only allowed on the master server", http.StatusForbidden)
		return
	}

	response, err := sendQueryToMaster(req.Query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
