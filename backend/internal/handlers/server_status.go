package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ServerStatusHandler struct {
	address string
}

type serverStatusResponse struct {
	Address string              `json:"address"`
	Online  bool                `json:"online"`
	Version string              `json:"version,omitempty"`
	Players serverStatusPlayers `json:"players"`
	Error   string              `json:"error,omitempty"`
}

type serverStatusPlayers struct {
	Online int `json:"online"`
	Max    int `json:"max"`
}

type minecraftStatusPayload struct {
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
}

func NewServerStatusHandler(address string) *ServerStatusHandler {
	address = strings.TrimSpace(address)
	if address == "" {
		address = "play.amy-world.ru"
	}
	return &ServerStatusHandler{address: address}
}

func (h *ServerStatusHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status, err := queryMinecraftStatus(ctx, h.address)
	if err != nil {
		writeJSON(w, http.StatusOK, serverStatusResponse{
			Address: displayMinecraftAddress(h.address),
			Online:  false,
			Players: serverStatusPlayers{},
			Error:   "server is not reachable",
		})
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func queryMinecraftStatus(ctx context.Context, rawAddress string) (serverStatusResponse, error) {
	host, port, err := splitMinecraftAddress(rawAddress)
	if err != nil {
		return serverStatusResponse{}, err
	}

	dialHost, dialPort := resolveMinecraftSRV(ctx, host, port)
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(dialHost, dialPort))
	if err != nil {
		return serverStatusResponse{}, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))

	if err := writeMinecraftHandshake(conn, host, port); err != nil {
		return serverStatusResponse{}, err
	}
	if err := writeMinecraftPacket(conn, []byte{0x00}); err != nil {
		return serverStatusResponse{}, err
	}

	reader := bufio.NewReader(conn)
	packetLength, err := readMinecraftVarInt(reader)
	if err != nil {
		return serverStatusResponse{}, err
	}
	if packetLength <= 0 || packetLength > 2*1024*1024 {
		return serverStatusResponse{}, fmt.Errorf("invalid packet length: %d", packetLength)
	}

	packetID, err := readMinecraftVarInt(reader)
	if err != nil {
		return serverStatusResponse{}, err
	}
	if packetID != 0 {
		return serverStatusResponse{}, fmt.Errorf("unexpected packet id: %d", packetID)
	}

	jsonLength, err := readMinecraftVarInt(reader)
	if err != nil {
		return serverStatusResponse{}, err
	}
	if jsonLength <= 0 || jsonLength > packetLength {
		return serverStatusResponse{}, fmt.Errorf("invalid json length: %d", jsonLength)
	}

	raw := make([]byte, jsonLength)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return serverStatusResponse{}, err
	}

	var payload minecraftStatusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return serverStatusResponse{}, err
	}

	return serverStatusResponse{
		Address: displayMinecraftAddress(rawAddress),
		Online:  true,
		Version: payload.Version.Name,
		Players: serverStatusPlayers{
			Online: payload.Players.Online,
			Max:    payload.Players.Max,
		},
	}, nil
}

func splitMinecraftAddress(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "minecraft://")
	if raw == "" {
		return "", "", fmt.Errorf("empty address")
	}

	if host, port, err := net.SplitHostPort(raw); err == nil {
		return strings.Trim(host, "[]"), port, nil
	}

	if strings.Count(raw, ":") == 1 {
		parts := strings.Split(raw, ":")
		if parsed, err := strconv.Atoi(parts[1]); err == nil && parsed > 0 && parsed <= 65535 {
			return parts[0], parts[1], nil
		}
	}

	return raw, "25565", nil
}

func displayMinecraftAddress(raw string) string {
	host, port, err := splitMinecraftAddress(raw)
	if err != nil {
		return "play.amy-world.ru"
	}
	if port == "25565" {
		return host
	}
	return net.JoinHostPort(host, port)
}

func resolveMinecraftSRV(ctx context.Context, host, port string) (string, string) {
	if port != "25565" || net.ParseIP(host) != nil {
		return host, port
	}

	_, addrs, err := net.DefaultResolver.LookupSRV(ctx, "minecraft", "tcp", host)
	if err != nil || len(addrs) == 0 {
		return host, port
	}

	target := strings.TrimSuffix(addrs[0].Target, ".")
	return target, strconv.Itoa(int(addrs[0].Port))
}

func writeMinecraftHandshake(w io.Writer, host, port string) error {
	payload := &bytes.Buffer{}
	writeMinecraftVarInt(payload, 0x00)
	writeMinecraftVarInt(payload, 767)
	writeMinecraftString(payload, host)

	portNumber, _ := strconv.Atoi(port)
	if portNumber <= 0 || portNumber > 65535 {
		portNumber = 25565
	}
	if err := binary.Write(payload, binary.BigEndian, uint16(portNumber)); err != nil {
		return err
	}

	writeMinecraftVarInt(payload, 0x01)
	return writeMinecraftPacket(w, payload.Bytes())
}

func writeMinecraftPacket(w io.Writer, payload []byte) error {
	packet := &bytes.Buffer{}
	writeMinecraftVarInt(packet, len(payload))
	packet.Write(payload)
	_, err := w.Write(packet.Bytes())
	return err
}

func writeMinecraftString(w io.Writer, value string) {
	raw := []byte(value)
	writeMinecraftVarInt(w, len(raw))
	_, _ = w.Write(raw)
}

func writeMinecraftVarInt(w io.Writer, value int) {
	for {
		if value&^0x7F == 0 {
			_, _ = w.Write([]byte{byte(value)})
			return
		}
		_, _ = w.Write([]byte{byte(value&0x7F | 0x80)})
		value >>= 7
	}
}

func readMinecraftVarInt(r io.ByteReader) (int, error) {
	value := 0
	for i := 0; i < 5; i++ {
		current, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		value |= int(current&0x7F) << (7 * i)
		if current&0x80 == 0 {
			return value, nil
		}
	}

	return 0, fmt.Errorf("varint is too large")
}
