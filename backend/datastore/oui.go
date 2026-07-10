package datastore

import (
	"embed"
	"encoding/csv"
	"encoding/hex"
	"io"
	"log"
	"strings"
)

//go:embed conf/mac-vendors-export.csv
var ouiCSV embed.FS

var ouiMap = make(map[string]string)

func init() {
	f, err := ouiCSV.Open("conf/mac-vendors-export.csv")
	if err != nil {
		log.Printf("failed to open embedded OUI CSV: %v", err)
		return
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Read header
	if _, err := r.Read(); err != nil {
		log.Printf("failed to read OUI CSV header: %v", err)
		return
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if len(record) < 2 {
			continue
		}
		oui := record[0]
		if !strings.Contains(oui, ":") {
			continue
		}
		oui = strings.TrimSpace(oui)
		oui = strings.ReplaceAll(oui, ":", "")
		oui = strings.ToUpper(oui)
		ouiMap[oui] = record[1]
	}
	log.Printf("loaded %d OUI vendor records", len(ouiMap))
}

// FindVendor resolves a MAC address to its manufacturer vendor name.
func FindVendor(mac string) string {
	if mac == "" {
		return ""
	}
	mac = strings.TrimSpace(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	if len(mac) >= 6 {
		mac = strings.ToUpper(mac)
		// Check 6 chars prefix
		if n, ok := ouiMap[mac[:6]]; ok {
			return n
		}
		// Check 7 chars prefix
		if len(mac) >= 7 {
			if n, ok := ouiMap[mac[:7]]; ok {
				return n
			}
		}
		// Check 9 chars prefix
		if len(mac) >= 9 {
			if n, ok := ouiMap[mac[:9]]; ok {
				return n
			}
		}
		// Check if it's a locally administered address
		if h, err := hex.DecodeString(mac); err == nil && len(h) > 0 {
			if (h[0] & 0x02) == 0x02 {
				// Local MAC address
				h[0] = h[0] & 0xfd
				mac = strings.ToUpper(hex.EncodeToString(h))
				if n, ok := ouiMap[mac[:6]]; ok {
					return n + " (Local)"
				}
				if len(mac) >= 7 {
					if n, ok := ouiMap[mac[:7]]; ok {
						return n + " (Local)"
					}
				}
				if len(mac) >= 9 {
					if n, ok := ouiMap[mac[:9]]; ok {
						return n + " (Local)"
					}
				}
				return "Local Address"
			}
		}
	}
	return "Unknown"
}
