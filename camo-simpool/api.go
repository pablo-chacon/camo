package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"
)

// simAPISocket is the Unix socket path exposed to other CAMO components.
const simAPISocket = "/run/camo/simpool.sock"

// simAPIRequest is an inbound command from another component.
type simAPIRequest struct {
	Command      string `json:"command"`
	SessionToken string `json:"session_token,omitempty"`
}

// simAPIResponse is the reply.
type simAPIResponse struct {
	Status       string `json:"status"`
	Command      string `json:"command"`
	IMSI         string `json:"imsi,omitempty"`
	Total        int    `json:"total,omitempty"`
	Active       int    `json:"active,omitempty"`
	Available    int    `json:"available,omitempty"`
	Message      string `json:"message,omitempty"`
}

const (
	simCmdAllocate  = "allocate"   // allocate a SIM for a session
	simCmdRelease   = "release"    // release a SIM from a session
	simCmdStats     = "stats"      // return pool statistics
)

// simAPIServer exposes the SIM pool over a Unix socket.
// Used by camo-circuit to request SIM allocation for circuit hops.
type simAPIServer struct {
	pool *pool
}

func newSIMAPIServer(p *pool) *simAPIServer {
	return &simAPIServer{pool: p}
}

func (s *simAPIServer) listen() error {
	_ = os.Remove(simAPISocket)
	if err := os.MkdirAll("/run/camo", 0700); err != nil {
		return err
	}
	ln, err := net.Listen("unix", simAPISocket)
	if err != nil {
		return err
	}
	slog.Info("simpool API listening", "socket", simAPISocket)
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("simpool API accept error", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *simAPIServer) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	enc := json.NewEncoder(conn)

	for scanner.Scan() {
		var req simAPIRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(simAPIResponse{Status: "error", Message: "invalid request"})
			continue
		}
		switch req.Command {
		case simCmdAllocate:
			imsi, err := s.pool.allocate(req.SessionToken)
			if err != nil {
				_ = enc.Encode(simAPIResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(simAPIResponse{Status: "ok", Command: req.Command, IMSI: imsi})

		case simCmdRelease:
			if err := s.pool.release(req.SessionToken); err != nil {
				_ = enc.Encode(simAPIResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(simAPIResponse{Status: "ok", Command: req.Command})

		case simCmdStats:
			total, active, available := s.pool.stats()
			_ = enc.Encode(simAPIResponse{
				Status:    "ok",
				Command:   req.Command,
				Total:     total,
				Active:    active,
				Available: available,
			})

		default:
			_ = enc.Encode(simAPIResponse{Status: "error", Message: "unknown command"})
		}
	}
}
