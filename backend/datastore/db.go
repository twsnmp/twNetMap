package datastore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketConfig = []byte("config")
	bucketNodes  = []byte("nodes")
	bucketLinks  = []byte("links")

	configKey = []byte("system")
)

// SnmpSetting represents a single SNMP credential configuration.
type SnmpSetting struct {
	Mode      string `json:"SnmpMode"`      // "v2c", "v3auth", "v3authpriv" etc.
	Community string `json:"SnmpCommunity"`
	User      string `json:"SnmpUser"`
	Password  string `json:"SnmpPassword"`
}

// Config represents the application settings.
type Config struct {
	Subnet          string        `json:"Subnet"`
	SnmpConfigs     []SnmpSetting `json:"SnmpConfigs"`
	Timeout         int           `json:"Timeout"`
	Retry           int           `json:"Retry"`
	ActiveProvider  string `json:"ActiveProvider"` // "openai", "ollama", "gemini"
	OllamaURL       string `json:"OllamaURL"`
	OllamaModel     string `json:"OllamaModel"`
	APIKeyOpenAI    string `json:"APIKeyOpenAI"`
	APIKeyGemini    string `json:"APIKeyGemini"`
	Language        string `json:"Language"` // "auto", "en", "ja"
}

// Node represents a network device.
type Node struct {
	ID             string  `json:"id"`
	IP             string  `json:"ip"`
	MAC            string  `json:"mac"`
	Vendor         string  `json:"vendor"`
	Label          string  `json:"label"`
	Type           string  `json:"type"` // "router", "switch", "pc", "server", "printer", "unknown"
	Reason         string  `json:"reason"`
	SysName        string  `json:"sysName"`
	SysDesc        string  `json:"sysDesc"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	ManuallyEdited bool    `json:"manuallyEdited"`
}

// Link represents a network link between two devices.
type Link struct {
	ID            string `json:"id"`
	From          string `json:"from"`
	To            string `json:"to"`
	Type          string `json:"type"` // e.g. "lan"
	ManuallyAdded bool   `json:"manuallyAdded"`
}

// NodeLinkData represents the combined nodes and links.
type NodeLinkData struct {
	Nodes []*Node `json:"nodes"`
	Links []*Link `json:"links"`
}

// DB wraps bbolt DB.
type DB struct {
	conn *bolt.DB
	mu   sync.Mutex
}

// NewDB opens the database and initializes buckets.
func NewDB(dbDir string) (*DB, error) {
	dbPath := filepath.Join(dbDir, "twnetmap.db")
	conn, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt db at %s: %w", dbPath, err)
	}

	db := &DB{conn: conn}
	if err := db.initBuckets(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

func (db *DB) initBuckets() error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{bucketConfig, bucketNodes, bucketLinks}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", b, err)
			}
		}
		return nil
	})
}

// GetConfig retrieves the system configuration.
func (db *DB) GetConfig() (*Config, error) {
	var cfg Config
	err := db.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		data := b.Get(configKey)
		if data == nil {
			// Return default config if not initialized
			cfg = Config{
				Subnet: "192.168.1.0/24",
				SnmpConfigs: []SnmpSetting{
					{
						Mode:      "v2c",
						Community: "public",
					},
				},
				Timeout:        3,
				Retry:          1,
				ActiveProvider: "ollama",
				OllamaURL:      "http://localhost:11434",
				OllamaModel:    "llama3",
				Language:       "auto",
			}
			return nil
		}
		return json.Unmarshal(data, &cfg)
	})
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig saves the system configuration.
func (db *DB) SaveConfig(cfg *Config) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put(configKey, data)
	})
}

// SaveNode saves/updates a node record.
func (db *DB) SaveNode(node *Node) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		data, err := json.Marshal(node)
		if err != nil {
			return err
		}
		return b.Put([]byte(node.ID), data)
	})
}

// SaveNodes saves multiple nodes in a single transaction.
func (db *DB) SaveNodes(nodes []*Node) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		for _, node := range nodes {
			data, err := json.Marshal(node)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(node.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteNode deletes a node.
func (db *DB) DeleteNode(id string) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		return b.Delete([]byte(id))
	})
}

// ClearAllNodes deletes all node records.
func (db *DB) ClearAllNodes() error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketNodes)
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(bucketNodes)
		return err
	})
}

// GetNodes lists all nodes.
func (db *DB) GetNodes() ([]*Node, error) {
	nodes := []*Node{}
	err := db.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var node Node
			if err := json.Unmarshal(v, &node); err != nil {
				return err
			}
			nodes = append(nodes, &node)
			return nil
		})
	})
	return nodes, err
}

// SaveLink saves/updates a link record.
func (db *DB) SaveLink(link *Link) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLinks)
		data, err := json.Marshal(link)
		if err != nil {
			return err
		}
		return b.Put([]byte(link.ID), data)
	})
}

// SaveLinks saves multiple links in a single transaction.
func (db *DB) SaveLinks(links []*Link) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLinks)
		for _, link := range links {
			data, err := json.Marshal(link)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(link.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteLink deletes a link.
func (db *DB) DeleteLink(id string) error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLinks)
		return b.Delete([]byte(id))
	})
}

// ClearAllLinks deletes all link records.
func (db *DB) ClearAllLinks() error {
	return db.conn.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketLinks)
		if err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(bucketLinks)
		return err
	})
}

// GetLinks lists all links.
func (db *DB) GetLinks() ([]*Link, error) {
	links := []*Link{}
	err := db.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLinks)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var link Link
			if err := json.Unmarshal(v, &link); err != nil {
				return err
			}
			links = append(links, &link)
			return nil
		})
	})
	return links, err
}
