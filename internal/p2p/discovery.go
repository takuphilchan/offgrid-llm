package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Peer represents a peer in the network
type Peer struct {
	ID       string
	Address  string
	Port     int
	LastSeen time.Time
	Models   []string
}

// Announcement represents a peer announcement message
type Announcement struct {
	NodeID  string   `json:"node_id"`
	Port    int      `json:"port"`
	Models  []string `json:"models"`
	Version string   `json:"version"`
}

// multicastGroup is the IPv4 multicast group for OffGrid discovery
// Uses a locally-scoped multicast address (239.x.x.x)
const multicastGroup = "239.255.77.77"

// Discovery handles peer discovery on the local network
type Discovery struct {
	mu            sync.RWMutex
	peers         map[string]*Peer
	localPort     int
	discoveryPort int
	enabled       bool
	stopChan      chan struct{}
	localModels   []string // Models available on this node
	nodeID        string   // Unique identifier for this node
}

// NewDiscovery creates a new P2P discovery instance
func NewDiscovery(localPort, discoveryPort int) *Discovery {
	// Generate truly unique node ID with random bytes
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	nodeID := fmt.Sprintf("node-%d-%s", time.Now().UnixNano(), hex.EncodeToString(randBytes))

	return &Discovery{
		peers:         make(map[string]*Peer),
		localPort:     localPort,
		discoveryPort: discoveryPort,
		enabled:       false,
		stopChan:      make(chan struct{}),
		localModels:   []string{},
		nodeID:        nodeID,
	}
}

// SetLocalModels updates the list of models available on this node
func (d *Discovery) SetLocalModels(models []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.localModels = models
}

// Start begins peer discovery
func (d *Discovery) Start(ctx context.Context) error {
	d.enabled = true

	// Start UDP listener for announcements
	go d.listenForAnnouncements(ctx)

	// Start periodic announcements
	go d.announcePresence(ctx)

	// Start peer cleanup
	go d.cleanupStale(ctx)

	log.Printf("[P2P] Discovery started on port %d", d.discoveryPort)
	return nil
}

// Stop stops peer discovery
func (d *Discovery) Stop() {
	d.enabled = false
	close(d.stopChan)
	log.Println("🛑 P2P Discovery stopped")
}

// GetPeers returns all known peers
func (d *Discovery) GetPeers() []*Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	peers := make([]*Peer, 0, len(d.peers))
	for _, peer := range d.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetNodeID returns the unique identifier for this node
func (d *Discovery) GetNodeID() string {
	return d.nodeID
}

// FindModelOnPeers searches for a model across peers
func (d *Discovery) FindModelOnPeers(modelID string) []*Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var peersWithModel []*Peer
	for _, peer := range d.peers {
		for _, model := range peer.Models {
			if model == modelID {
				peersWithModel = append(peersWithModel, peer)
				break
			}
		}
	}
	return peersWithModel
}

// listenForAnnouncements listens for peer announcements on UDP multicast
func (d *Discovery) listenForAnnouncements(ctx context.Context) {
	// Join multicast group for discovery
	multicastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", multicastGroup, d.discoveryPort))
	if err != nil {
		log.Printf("Failed to resolve multicast address: %v", err)
		return
	}

	// Listen on all interfaces
	conn, err := net.ListenMulticastUDP("udp", nil, multicastAddr)
	if err != nil {
		// Fallback to regular UDP if multicast fails (e.g., no multicast support)
		log.Printf("Multicast not available, falling back to broadcast: %v", err)
		d.listenBroadcast(ctx)
		return
	}
	defer conn.Close()

	// Set read buffer size
	conn.SetReadBuffer(4096)

	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				continue
			}

			// Parse announcement
			d.handleAnnouncement(string(buffer[:n]), addr.IP.String())
		}
	}
}

// listenBroadcast is a fallback listener for systems without multicast
func (d *Discovery) listenBroadcast(ctx context.Context) {
	addr := net.UDPAddr{
		Port: d.discoveryPort,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Printf("Failed to start UDP listener: %v", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Error reading UDP: %v", err)
				continue
			}

			// Parse announcement
			d.handleAnnouncement(string(buffer[:n]), addr.IP.String())
		}
	}
}

// announcePresence periodically announces this node's presence
func (d *Discovery) announcePresence(ctx context.Context) {
	// Immediate broadcast on start
	d.broadcast()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.broadcast()
		}
	}
}

// broadcast sends an announcement to the local network via multicast
func (d *Discovery) broadcast() {
	// Use multicast address for discovery
	multicastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", multicastGroup, d.discoveryPort))
	if err != nil {
		log.Printf("Failed to resolve multicast address: %v", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		// Fallback to broadcast if multicast fails
		d.broadcastFallback()
		return
	}
	defer conn.Close()

	// Create JSON announcement
	d.mu.RLock()
	announcement := Announcement{
		NodeID:  d.nodeID,
		Port:    d.localPort,
		Models:  d.localModels,
		Version: "0.2.12",
	}
	d.mu.RUnlock()

	data, err := json.Marshal(announcement)
	if err != nil {
		log.Printf("Failed to marshal announcement: %v", err)
		return
	}

	_, err = conn.Write(data)
	if err != nil {
		log.Printf("Failed to multicast: %v", err)
	}
}

// broadcastFallback uses traditional broadcast for systems without multicast
func (d *Discovery) broadcastFallback() {
	addr := net.UDPAddr{
		Port: d.discoveryPort,
		IP:   net.IPv4bcast,
	}

	conn, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		log.Printf("Failed to create broadcast connection: %v", err)
		return
	}
	defer conn.Close()

	// Create JSON announcement
	d.mu.RLock()
	announcement := Announcement{
		NodeID:  d.nodeID,
		Port:    d.localPort,
		Models:  d.localModels,
		Version: "0.2.12",
	}
	d.mu.RUnlock()

	data, err := json.Marshal(announcement)
	if err != nil {
		log.Printf("Failed to marshal announcement: %v", err)
		return
	}

	_, err = conn.Write(data)
	if err != nil {
		log.Printf("Failed to broadcast: %v", err)
	}
}

// handleAnnouncement processes a peer announcement
func (d *Discovery) handleAnnouncement(message, fromIP string) {
	// Parse JSON announcement
	var announcement Announcement
	if err := json.Unmarshal([]byte(message), &announcement); err != nil {
		// Ignore malformed announcements
		return
	}

	// Don't add ourselves
	if announcement.NodeID == d.nodeID {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	peerID := announcement.NodeID
	if peer, exists := d.peers[peerID]; exists {
		// Update existing peer
		peer.LastSeen = time.Now()
		peer.Port = announcement.Port
		peer.Models = announcement.Models
	} else {
		// Add new peer
		d.peers[peerID] = &Peer{
			ID:       peerID,
			Address:  fromIP,
			Port:     announcement.Port,
			LastSeen: time.Now(),
			Models:   announcement.Models,
		}
		log.Printf("🌐 Discovered new peer: %s (%s:%d) with %d models",
			peerID, fromIP, announcement.Port, len(announcement.Models))
	}
}

// cleanupStale removes peers that haven't been seen recently
func (d *Discovery) cleanupStale(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.mu.Lock()
			now := time.Now()
			for id, peer := range d.peers {
				if now.Sub(peer.LastSeen) > 2*time.Minute {
					delete(d.peers, id)
					log.Printf("Removed stale peer: %s", id)
				}
			}
			d.mu.Unlock()
		}
	}
}
