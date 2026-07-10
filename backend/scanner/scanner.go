package scanner

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"twNetMap/backend/datastore"
)

// ScanResult represents the raw collected data for a single node.
type ScanResult struct {
	IP          string            `json:"ip"`
	MAC         string            `json:"mac"`
	Vendor      string            `json:"vendor"`
	SysName     string            `json:"sysName"`
	SysDesc     string            `json:"sysDesc"`
	OpenPorts   []int             `json:"openPorts"`
	LLDPNeighbors []LLDPNeighbor   `json:"lldpNeighbors"`
}

type LLDPNeighbor struct {
	ChassisID string `json:"chassisId"`
	PortID    string `json:"portId"`
	SysName   string `json:"sysName"`
	SysDesc   string `json:"sysDesc"`
	IP        string `json:"ip"`
}

// GenerateIPs parses a comma-separated list of targets (CIDR subnets, IP ranges, or single IPs).
func GenerateIPs(target string) ([]string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("empty scan target")
	}

	var allIPs []string
	seen := make(map[string]bool)

	parts := strings.Split(target, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ips, err := generateIPsSingle(part)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	if len(allIPs) == 0 {
		return nil, fmt.Errorf("no valid target IPs found")
	}

	// Limit total combined target IPs to a reasonable number (e.g., 1024) for safety
	if len(allIPs) > 1024 {
		return nil, fmt.Errorf("total number of target IPs exceeds limit of 1024 (got %d)", len(allIPs))
	}

	return allIPs, nil
}

func generateIPsSingle(target string) ([]string, error) {
	// Case 1: IP range like 192.168.1.1-192.168.1.50
	if strings.Contains(target, "-") {
		parts := strings.Split(target, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}
		startIPStr := strings.TrimSpace(parts[0])
		endIPStr := strings.TrimSpace(parts[1])

		startIP := net.ParseIP(startIPStr)
		endIP := net.ParseIP(endIPStr)
		if startIP == nil || endIP == nil {
			return nil, fmt.Errorf("invalid range IP addresses")
		}

		startIP = startIP.To4()
		endIP = endIP.To4()
		if startIP == nil || endIP == nil {
			return nil, fmt.Errorf("only IPv4 range is supported")
		}

		var ips []string
		startVal := ipToUint32(startIP)
		endVal := ipToUint32(endIP)
		if startVal > endVal {
			return nil, fmt.Errorf("start IP is greater than end IP")
		}

		// Limit range size to a /24 equivalent (256 addresses) for safety
		if endVal-startVal > 512 {
			return nil, fmt.Errorf("range is too large (maximum 512 addresses)")
		}

		for val := startVal; val <= endVal; val++ {
			ips = append(ips, uint32ToIP(val).String())
		}
		return ips, nil
	}

	// Case 2: CIDR like 192.168.1.0/24
	if strings.Contains(target, "/") {
		_, ipNet, err := net.ParseCIDR(target)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %w", err)
		}

		ones, bits := ipNet.Mask.Size()
		if bits-ones > 10 { // Max size /22 (1024 addresses)
			return nil, fmt.Errorf("subnet mask is too large (maximum /22)")
		}

		var ips []string
		ip := ipNet.IP.Mask(ipNet.Mask)
		for {
			ips = append(ips, ip.String())
			if !incrementIP(ip) || !ipNet.Contains(ip) {
				break
			}
		}

		// Remove network and broadcast address if subnet is larger than /31
		if len(ips) > 2 {
			return ips[1 : len(ips)-1], nil
		}
		return ips, nil
	}

	// Case 3: Single IP
	ip := net.ParseIP(target)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format: %s", target)
	}
	return []string{ip.String()}, nil
}

func ipToUint32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(val uint32) net.IP {
	return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
}

func incrementIP(ip net.IP) bool {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			return true
		}
	}
	return false
}

// Ping checks if host responds to ICMP.
// Uses unprivileged UDP ping, falls back to exec ping command.
func Ping(ip string, timeout time.Duration) bool {
	if tryUDPPing(ip, timeout) {
		return true
	}
	return execPingCmd(ip, timeout)
}

func tryUDPPing(ipStr string, timeout time.Duration) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return false
	}
	defer c.Close()

	c.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true)

	body := &icmp.Echo{
		ID:   1,
		Seq:  1,
		Data: []byte("twNetMap-ping"),
	}
	msg := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: body,
	}

	b, err := msg.Marshal(nil)
	if err != nil {
		return false
	}

	dst := &net.UDPAddr{IP: ip}
	if _, err := c.WriteTo(b, dst); err != nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	reply := make([]byte, 1500)
	for {
		if time.Now().After(deadline) {
			return false
		}
		c.SetReadDeadline(deadline)
		n, peer, err := c.ReadFrom(reply)
		if err != nil {
			return false
		}

		udpPeer, ok := peer.(*net.UDPAddr)
		if !ok || udpPeer.IP.String() != ipStr {
			continue // ignore replies from other hosts (late replies on reused ports)
		}

		parsed, err := icmp.ParseMessage(protocolICMP, reply[:n])
		if err != nil {
			continue
		}

		if parsed.Type == ipv4.ICMPTypeEchoReply {
			return true
		}
	}
}

const protocolICMP = 1

func execPingCmd(ip string, timeout time.Duration) bool {
	var cmd *exec.Cmd
	timeoutMs := fmt.Sprintf("%d", timeout.Milliseconds())
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", timeoutMs, ip)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("ping", "-c", "1", "-t", strconv.Itoa(int(timeout.Seconds())), "-W", timeoutMs, ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", strconv.Itoa(int(timeout.Seconds())), ip)
	}

	err := cmd.Run()
	return err == nil
}

// GetARPTable retrieves system ARP mappings.
func GetARPTable() map[string]string {
	arpMap := make(map[string]string)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("arp", "-a")
	} else {
		cmd = exec.Command("arp", "-an")
	}

	out, err := cmd.Output()
	if err != nil {
		log.Printf("failed to run arp command: %v", err)
		return arpMap
	}

	ipRegex := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	macRegex := regexp.MustCompile(`([0-9a-fA-F]{1,2}[:-]){5}([0-9a-fA-F]{1,2})`)

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		ip := ipRegex.FindString(line)
		mac := macRegex.FindString(line)
		if ip != "" && mac != "" {
			// Normalize MAC format
			mac = strings.ReplaceAll(mac, "-", ":")
			mac = strings.ToLower(mac)
			// Format single-digit components like 0:11:32... to 00:11:32...
			parts := strings.Split(mac, ":")
			for i, p := range parts {
				if len(p) == 1 {
					parts[i] = "0" + p
				}
			}
			arpMap[ip] = strings.Join(parts, ":")
		}
	}
	return arpMap
}

// ScanPorts scans common TCP ports.
func ScanPorts(ip string, ports []int, timeout time.Duration) []int {
	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan bool, 10) // Limit concurrency to 10 ports at a time

	for _, port := range ports {
		wg.Add(1)
		sem <- true
		go func(p int) {
			defer func() {
				<-sem
				wg.Done()
			}()
			addr := fmt.Sprintf("%s:%d", ip, p)
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()
	return openPorts
}

// QuerySNMP gets SysName, SysDesc and runs LLDP walks if SNMP is available.
func QuerySNMP(ip, community, mode, user, password string, timeoutSec, retry int) (string, string, []LLDPNeighbor) {
	agent := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Retries:   retry,
	}

	if mode == "v3auth" || mode == "v3authpriv" {
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthNoPriv
		if mode == "v3authpriv" {
			agent.MsgFlags = gosnmp.AuthPriv
		}
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 user,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        password,
		}
	}

	err := agent.Connect()
	if err != nil {
		return "", "", nil
	}
	defer agent.Conn.Close()

	sysName := ""
	sysDesc := ""

	// Get SysName and SysDesc
	result, err := agent.Get([]string{".1.3.6.1.2.1.1.5.0", ".1.3.6.1.2.1.1.1.0"})
	if err == nil {
		for _, variable := range result.Variables {
			switch variable.Name {
			case ".1.3.6.1.2.1.1.5.0":
				sysName = getStringValue(variable.Value)
			case ".1.3.6.1.2.1.1.1.0":
				sysDesc = getStringValue(variable.Value)
			}
		}
	}

	// Walk LLDP MIB
	var neighbors []LLDPNeighbor
	neighborMap := make(map[string]*LLDPNeighbor)

	// Walk lldpRemoteSystemsData (.1.0.8802.1.1.2.1.4)
	err = agent.Walk(".1.0.8802.1.1.2.1.4", func(variable gosnmp.SnmpPDU) error {
		name := variable.Name
		// Extract neighbor index from OID suffix
		// Standard LLDP Remote tables OID is like:
		// .1.0.8802.1.1.2.1.4.1.1.X.<timeMark>.<localPortNum>.<remIndex>
		parts := strings.Split(strings.TrimPrefix(name, ".1.0.8802.1.1.2.1.4.1.1."), ".")
		if len(parts) >= 4 {
			oidType := parts[0]
			indexKey := strings.Join(parts[1:], ".")
			if _, ok := neighborMap[indexKey]; !ok {
				neighborMap[indexKey] = &LLDPNeighbor{}
			}
			valStr := getStringValue(variable.Value)
			switch oidType {
			case "5": // lldpRemChassisId
				neighborMap[indexKey].ChassisID = valStr
			case "7": // lldpRemPortId
				neighborMap[indexKey].PortID = valStr
			case "9": // lldpRemSysName
				neighborMap[indexKey].SysName = valStr
			case "10": // lldpRemSysDesc
				neighborMap[indexKey].SysDesc = valStr
			}
		}

		// Also extract management addresses from lldpRemManAddrEntry (.1.0.8802.1.1.2.1.4.2.1)
		// OID: .1.0.8802.1.1.2.1.4.2.1.4.<timeMark>.<localPortNum>.<remIndex>.<addrSubtype>.<addrLength>.<addr...>
		// Where the address is encoded in the OID suffix
		if strings.HasPrefix(name, ".1.0.8802.1.1.2.1.4.2.1.4.") {
			suffix := strings.TrimPrefix(name, ".1.0.8802.1.1.2.1.4.2.1.4.")
			addrParts := strings.Split(suffix, ".")
			// Typically: <timeMark>.<localPortNum>.<remIndex>.<addrSubtype>.<addrLength>.<ipParts...>
			// For IPv4, subtype is 1 (ipv4), length is 4.
			if len(addrParts) >= 9 {
				indexKey := strings.Join(addrParts[:3], ".")
				subType := addrParts[3]
				addrLen := addrParts[4]
				if subType == "1" && addrLen == "4" && len(addrParts) >= 9 {
					ip := strings.Join(addrParts[5:9], ".")
					if _, ok := neighborMap[indexKey]; !ok {
						neighborMap[indexKey] = &LLDPNeighbor{}
					}
					neighborMap[indexKey].IP = ip
				}
			}
		}

		return nil
	})

	for _, n := range neighborMap {
		if n.ChassisID != "" || n.SysName != "" {
			neighbors = append(neighbors, *n)
		}
	}

	return sysName, sysDesc, neighbors
}

func getStringValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// PerformScan executes the scanning workflow for a target subnet/range.
func PerformScan(ctx context.Context, target string, cfg *datastore.Config, progressCallback func(percent int, msg string), onDetectCallback func(result *ScanResult)) ([]*ScanResult, error) {
	ips, err := GenerateIPs(target)
	if err != nil {
		return nil, err
	}

	progressCallback(0, fmt.Sprintf("Generating range: found %d IP addresses to scan", len(ips)))

	// First query the ARP table to get existing MAC mappings
	progressCallback(5, "Reading local ARP table...")
	arpTable := GetARPTable()

	var results []*ScanResult
	var mu sync.Mutex

	total := len(ips)
	concurrency := 20
	sem := make(chan bool, concurrency)
	var wg sync.WaitGroup

	portsToScan := []int{21, 22, 23, 80, 161, 443, 3306, 3389, 5432, 8080, 9100}

	for i, ipStr := range ips {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- true

		go func(index int, ip string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// 1. Ping
			timeout := time.Duration(cfg.Timeout) * time.Second
			alive := Ping(ip, timeout)
			if !alive {
				// We still try to resolve it from the ARP table.
				// Sometimes devices respond to ARP but block ICMP.
				if _, exists := arpTable[ip]; !exists {
					return
				}
			}

			// 2. ARP and Vendor OUI
			mac := arpTable[ip]
			vendor := datastore.FindVendor(mac)

			// 3. Port Scan
			openPorts := ScanPorts(ip, portsToScan, timeout)

			// 4. SNMP/LLDP
			sysName, sysDesc, neighbors := QuerySNMP(ip, cfg.SnmpCommunity, cfg.SnmpMode, cfg.SnmpUser, cfg.SnmpPassword, cfg.Timeout, cfg.Retry)

			res := &ScanResult{
				IP:            ip,
				MAC:           mac,
				Vendor:        vendor,
				SysName:       sysName,
				SysDesc:       sysDesc,
				OpenPorts:     openPorts,
				LLDPNeighbors: neighbors,
			}

			mu.Lock()
			results = append(results, res)
			pct := 5 + int(float64(index+1)/float64(total)*90.0)
			progressCallback(pct, fmt.Sprintf("Scanned %s - Alive (Ports: %v, SNMP: %v)", ip, openPorts, sysName != ""))
			if onDetectCallback != nil {
				onDetectCallback(res)
			}
			mu.Unlock()

		}(i, ipStr)
	}

	wg.Wait()
	progressCallback(100, fmt.Sprintf("Scan completed: %d active devices identified", len(results)))
	return results, nil
}
