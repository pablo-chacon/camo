package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"
)

// apnSocket is the Unix socket path.
// Spec reference: Appendix B — /run/apncore.sock
const apnSocket = "/run/apncore.sock"

// apiRequest mirrors Appendix B.2 command format.
type apiRequest struct {
	Command            string `json:"command"`
	IMSI               string `json:"imsi,omitempty"`
	NextHopWireGuardIP string `json:"next_hop_wireguard_ip,omitempty"`
	SessionToken       string `json:"session_token,omitempty"`
}

// apiResponse mirrors Appendix B.3 response format.
type apiResponse struct {
	Status   string    `json:"status"`
	Command  string    `json:"command"`
	Message  string    `json:"message,omitempty"`
	Sessions []Session `json:"sessions,omitempty"`
}

const (
	cmdSetRoute    = "set_route"
	cmdClearRoute  = "clear_route"
	cmdListSessions = "list_sessions"
)

// apiServer implements the APN core management API from Appendix B.
// Translates incoming commands to APNAdapter calls and forwards
// session events to camo-circuit.
type apiServer struct {
	adapter APNAdapter
}

func newAPIServer(adapter APNAdapter) *apiServer {
	return &apiServer{adapter: adapter}
}

// listen starts the Unix socket server.
// Spec reference: Appendix B — Unix socket at /run/apncore.sock
func (s *apiServer) listen() error {
	_ = os.Remove(apnSocket)
	ln, err := net.Listen("unix", apnSocket)
	if err != nil {
		return err
	}
	slog.Info("apncore API listening", "socket", apnSocket)

	// Forward session events to all connected listeners.
	// In the reference implementation camo-circuit connects
	// and receives events over a persistent connection.
	go s.forwardEvents()

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("apncore API accept error", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *apiServer) handleConn(conn net.Conn) {
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
		case cmdSetRoute:
			err := s.adapter.SetRoute(RouteRule{
				IMSI:               req.IMSI,
				NextHopWireGuardIP: req.NextHopWireGuardIP,
				SessionToken:       req.SessionToken,
			})
			if err != nil {
				_ = enc.Encode(apiResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(apiResponse{Status: "ok", Command: req.Command})

		case cmdClearRoute:
			err := s.adapter.ClearRoute(req.IMSI)
			if err != nil {
				_ = enc.Encode(apiResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(apiResponse{Status: "ok", Command: req.Command})

		case cmdListSessions:
			sessions, err := s.adapter.ListSessions()
			if err != nil {
				_ = enc.Encode(apiResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(apiResponse{
				Status:   "ok",
				Command:  req.Command,
				Sessions: sessions,
			})

		default:
			_ = enc.Encode(apiResponse{Status: "error", Command: req.Command, Message: "unknown command"})
		}
	}
}

// forwardEvents reads from the adapter's event channel and logs events.
// In a full implementation this would push events to camo-circuit
// over a persistent Unix socket connection.
func (s *apiServer) forwardEvents() {
	for event := range s.adapter.SessionEvents() {
		slog.Info("session event",
			"type", event.Type,
			"imsi", event.Session.IMSI,
			"ip", event.Session.DeviceIP,
		)
	}
}
