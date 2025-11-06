package p2p

import (
	"context"
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

// Discovery handles peer discovery on the local network
type Discovery struct {
	mu            sync.RWMutex
	peers         map[string]*Peer
	localPort     int
	discoveryPort int
	enabled       bool
	stopChan      chan struct{}
}

// NewDiscovery creates a new P2P discovery instance
func NewDiscovery(localPort, discoveryPort int) *Discovery {
	return &Discovery{
		peers:         make(map[string]*Peer),
		localPort:     localPort,
		discoveryPort: discoveryPort,
		enabled:       false,
		stopChan:      make(chan struct{}),
	}
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

	log.Printf("🔍 P2P Discovery started on port %d", d.discoveryPort)
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

// listenForAnnouncements listens for peer announcements on UDP
func (d *Discovery) listenForAnnouncements(ctx context.Context) {
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

// broadcast sends an announcement to the local network
func (d *Discovery) broadcast() {
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

	// Simple announcement format: "OFFGRID:PORT:MODELS"
	message := fmt.Sprintf("OFFGRID:%d", d.localPort)
	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Printf("Failed to broadcast: %v", err)
	}
}

// handleAnnouncement processes a peer announcement
func (d *Discovery) handleAnnouncement(message, fromIP string) {
	// TODO: Parse announcement and update peer list
	// For now, just log it
	log.Printf("Received announcement from %s: %s", fromIP, message)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Simple peer tracking
	peerID := fromIP
	if peer, exists := d.peers[peerID]; exists {
		peer.LastSeen = time.Now()
	} else {
		d.peers[peerID] = &Peer{
			ID:       peerID,
			Address:  fromIP,
			LastSeen: time.Now(),
			Models:   []string{},
		}
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
