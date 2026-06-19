package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"
)

// apiSocket is the path of the Unix socket exposed to other CAMO components.
const apiSocket = "/run/camo/gossip.sock"

// apiRequest is an inbound query from another component.
type apiRequest struct {
	Command  string `json:"command"`
	ServerID string `json:"server_id,omitempty"` // for get_node
}

// apiResponse is the reply sent back over the Unix socket.
type apiResponse struct {
	Status  string        `json:"status"`
	Command string        `json:"command"`
	Records []*NodeRecord `json:"records,omitempty"`
	Record  *NodeRecord   `json:"record,omitempty"`
	Message string        `json:"message,omitempty"`
}

const (
	cmdListPeers = "list_peers" // returns all active NodeRecords
	cmdGetNode   = "get_node"   // returns a specific NodeRecord by server_id
	cmdNodeInfo  = "node_info"  // returns this node's own NodeRecord
)

// apiServer exposes a Unix socket API for other CAMO components to query
// the gossip state — primarily for circuit construction.
type apiServer struct {
	store      *store
	ownRecord  *NodeRecord
}

func newAPIServer(st *store, own *NodeRecord) *apiServer {
	return &apiServer{store: st, ownRecord: own}
}

// listen starts the Unix socket API server.
func (a *apiServer) listen() error {
	// Remove stale socket file.
	_ = os.Remove(apiSocket)

	if err := os.MkdirAll("/run/camo", 0700); err != nil {
		return err
	}

	ln, err := net.Listen("unix", apiSocket)
	if err != nil {
		return err
	}
	slog.Info("gossip API listening", "socket", apiSocket)

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("gossip API accept error", "err", err)
			continue
		}
		go a.handleConn(conn)
	}
}

func (a *apiServer) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	enc := json.NewEncoder(conn)

	for scanner.Scan() {
		var req apiRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(apiResponse{Status: "error", Message: "invalid request"})
			continue
		}

		switch req.Command {
		case cmdListPeers:
			records := a.store.activeRecords()
			_ = enc.Encode(apiResponse{
				Status:  "ok",
				Command: cmdListPeers,
				Records: records,
			})

		case cmdGetNode:
			record := a.store.get(req.ServerID)
			if record == nil {
				_ = enc.Encode(apiResponse{
					Status:  "error",
					Command: cmdGetNode,
					Message: "not found",
				})
				continue
			}
			_ = enc.Encode(apiResponse{
				Status:  "ok",
				Command: cmdGetNode,
				Record:  record,
			})

		case cmdNodeInfo:
			_ = enc.Encode(apiResponse{
				Status:  "ok",
				Command: cmdNodeInfo,
				Record:  a.ownRecord,
			})

		default:
			_ = enc.Encode(apiResponse{
				Status:  "error",
				Command: req.Command,
				Message: "unknown command",
			})
		}
	}
}
