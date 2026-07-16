package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
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
	IP            string            `json:"ip"`
	MAC           string            `json:"mac"`
	Vendor        string            `json:"vendor"`
	SysName       string            `json:"sysName"`
	SysDesc       string            `json:"sysDesc"`
	OpenPorts     []int             `json:"openPorts"`
	LLDPNeighbors []LLDPNeighbor    `json:"lldpNeighbors"`
	Banners       map[string]string `json:"banners,omitempty"`
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

func newCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	setupCmd(cmd)
	return cmd
}

func execPingCmd(ip string, timeout time.Duration) bool {
	var cmd *exec.Cmd
	timeoutMs := fmt.Sprintf("%d", timeout.Milliseconds())
	if runtime.GOOS == "windows" {
		cmd = newCommand("ping", "-n", "1", "-w", timeoutMs, ip)
	} else if runtime.GOOS == "darwin" {
		cmd = newCommand("ping", "-c", "1", "-t", strconv.Itoa(int(timeout.Seconds())), "-W", timeoutMs, ip)
	} else {
		cmd = newCommand("ping", "-c", "1", "-W", strconv.Itoa(int(timeout.Seconds())), ip)
	}

	err := cmd.Run()
	return err == nil
}

// GetARPTable retrieves system ARP mappings.
func GetARPTable() map[string]string {
	arpMap := make(map[string]string)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = newCommand("arp", "-a")
	} else {
		cmd = newCommand("arp", "-an")
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

func getIntValue(val interface{}) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	default:
		return 0
	}
}

func formatMAC(bytes []byte) string {
	parts := make([]string, len(bytes))
	for i, b := range bytes {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

func getMACFromSNMP(agent *gosnmp.GoSNMP, targetIP string) string {
	// 1. Try to get the interface index for the target IP
	// OID: .1.3.6.1.2.1.4.20.1.2.<targetIP>
	ipIfIndexOid := fmt.Sprintf(".1.3.6.1.2.1.4.20.1.2.%s", targetIP)
	result, err := agent.Get([]string{ipIfIndexOid})
	if err == nil && len(result.Variables) > 0 && result.Variables[0].Value != nil {
		ifIndex := getIntValue(result.Variables[0].Value)
		if ifIndex > 0 {
			// 2. Get the MAC address for this interface index
			// OID: .1.3.6.1.2.1.2.2.1.6.<ifIndex>
			macOid := fmt.Sprintf(".1.3.6.1.2.1.2.2.1.6.%d", ifIndex)
			macResult, err := agent.Get([]string{macOid})
			if err == nil && len(macResult.Variables) > 0 && macResult.Variables[0].Value != nil {
				val := macResult.Variables[0].Value
				if bytes, ok := val.([]byte); ok && len(bytes) == 6 {
					return formatMAC(bytes)
				}
			}
		}
	}

	// Fallback 1: Walk ipAdEntIfIndex to build mapping
	ipToIfIndex := make(map[string]int64)
	_ = agent.Walk(".1.3.6.1.2.1.4.20.1.2", func(variable gosnmp.SnmpPDU) error {
		name := variable.Name
		if strings.HasPrefix(name, ".1.3.6.1.2.1.4.20.1.2.") {
			ip := strings.TrimPrefix(name, ".1.3.6.1.2.1.4.20.1.2.")
			val := getIntValue(variable.Value)
			ipToIfIndex[ip] = val
		}
		return nil
	})

	if ifIndex, ok := ipToIfIndex[targetIP]; ok && ifIndex > 0 {
		macOid := fmt.Sprintf(".1.3.6.1.2.1.2.2.1.6.%d", ifIndex)
		macResult, err := agent.Get([]string{macOid})
		if err == nil && len(macResult.Variables) > 0 && macResult.Variables[0].Value != nil {
			val := macResult.Variables[0].Value
			if bytes, ok := val.([]byte); ok && len(bytes) == 6 {
				return formatMAC(bytes)
			}
		}
	}

	// Fallback 2: Walk ifPhysAddress and return the first valid non-loopback MAC
	var fallbackMAC string
	_ = agent.Walk(".1.3.6.1.2.1.2.2.1.6", func(variable gosnmp.SnmpPDU) error {
		if bytes, ok := variable.Value.([]byte); ok && len(bytes) == 6 {
			isZero := true
			for _, b := range bytes {
				if b != 0 {
					isZero = false
					break
				}
			}
			if !isZero && fallbackMAC == "" {
				fallbackMAC = formatMAC(bytes)
			}
		}
		return nil
	})

	return fallbackMAC
}

func getArpTableFromSNMP(agent *gosnmp.GoSNMP) map[string]string {
	arpMap := make(map[string]string)

	// Walk ipNetToMediaPhysAddress (.1.3.6.1.2.1.4.22.1.2)
	_ = agent.Walk(".1.3.6.1.2.1.4.22.1.2", func(variable gosnmp.SnmpPDU) error {
		name := variable.Name
		if strings.HasPrefix(name, ".1.3.6.1.2.1.4.22.1.2.") {
			suffix := strings.TrimPrefix(name, ".1.3.6.1.2.1.4.22.1.2.")
			parts := strings.Split(suffix, ".")
			if len(parts) >= 5 {
				ip := strings.Join(parts[len(parts)-4:], ".")
				if bytes, ok := variable.Value.([]byte); ok && len(bytes) == 6 {
					isZero := true
					for _, b := range bytes {
						if b != 0 {
							isZero = false
							break
						}
					}
					if !isZero {
						arpMap[ip] = formatMAC(bytes)
					}
				}
			}
		}
		return nil
	})

	// Walk ipNetToPhysicalPhysAddress (.1.3.6.1.2.1.4.35.1.4)
	_ = agent.Walk(".1.3.6.1.2.1.4.35.1.4", func(variable gosnmp.SnmpPDU) error {
		name := variable.Name
		if strings.HasPrefix(name, ".1.3.6.1.2.1.4.35.1.4.") {
			suffix := strings.TrimPrefix(name, ".1.3.6.1.2.1.4.35.1.4.")
			parts := strings.Split(suffix, ".")
			if len(parts) >= 7 {
				addrType := parts[1]
				addrLength := parts[2]
				if addrType == "1" && addrLength == "4" {
					ip := strings.Join(parts[3:], ".")
					if bytes, ok := variable.Value.([]byte); ok && len(bytes) == 6 {
						isZero := true
						for _, b := range bytes {
							if b != 0 {
								isZero = false
								break
							}
						}
						if !isZero {
							arpMap[ip] = formatMAC(bytes)
						}
					}
				}
			}
		}
		return nil
	})

	return arpMap
}

// QuerySNMP gets SysName, SysDesc, MAC address, ARP table, and runs LLDP walks if SNMP is available.
// It also queries tcpConnTable to get local open listening TCP ports.
func QuerySNMP(ip, community, mode, user, password string, timeoutSec, retry int) (string, string, string, map[string]string, []LLDPNeighbor, []int) {
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
		return "", "", "", nil, nil, nil
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

	// Retrieve target's own MAC address
	mac := getMACFromSNMP(agent, ip)

	// Retrieve target's ARP/ND table (containing mappings for other nodes)
	snmpArp := getArpTableFromSNMP(agent)

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

	// Retrieve open TCP ports via tcpConnTable (.1.3.6.1.2.1.6.13.1.1)
	var openPorts []int
	openPortsMap := make(map[int]bool)
	_ = agent.Walk(".1.3.6.1.2.1.6.13.1.1", func(variable gosnmp.SnmpPDU) error {
		name := variable.Name
		state := getIntValue(variable.Value)
		if state == 2 { // 2 = listen
			suffix := strings.TrimPrefix(name, ".1.3.6.1.2.1.6.13.1.1.")
			parts := strings.Split(suffix, ".")
			if len(parts) >= 10 {
				if p, err := strconv.Atoi(parts[4]); err == nil {
					openPortsMap[p] = true
				}
			}
		}
		return nil
	})
	for p := range openPortsMap {
		openPorts = append(openPorts, p)
	}

	return sysName, sysDesc, mac, snmpArp, neighbors, openPorts
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
	progressCallback(3, "Reading local ARP table...")
	localArpTable := GetARPTable()

	// フェーズ1：PINGによるIPアドレス収集
	progressCallback(5, "Running Ping sweep...")

	var pingAliveMu sync.Mutex
	pingAliveIPs := make(map[string]bool)

	concurrency := 20
	if cfg.PortScanMode == "off" || cfg.PortScanMode == "safe" {
		concurrency = 10
	}

	sem := make(chan bool, concurrency)
	var wg sync.WaitGroup

	total := len(ips)
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

			timeout := time.Duration(cfg.Timeout) * time.Second
			if Ping(ip, timeout) {
				pingAliveMu.Lock()
				pingAliveIPs[ip] = true
				pingAliveMu.Unlock()

				// PINGで応答があったものは即座にマップに追加（通知）
				if onDetectCallback != nil {
					mac := localArpTable[ip]
					vendor := datastore.FindVendor(mac)
					onDetectCallback(&ScanResult{
						IP:     ip,
						MAC:    mac,
						Vendor: vendor,
					})
				}
			}
			// Progress 5% to 40%
			pct := 5 + int(float64(index+1)/float64(total)*35.0)
			progressCallback(pct, fmt.Sprintf("Pinged %s", ip))
		}(i, ipStr)
	}
	wg.Wait()

	progressCallback(40, "Querying SNMP for ping-alive hosts...")

	type HostInfo struct {
		IP            string
		MAC           string
		SysName       string
		SysDesc       string
		SNMPActive    bool
		SNMPPorts     []int
		LLDPNeighbors []LLDPNeighbor
	}

	var hostInfos []*HostInfo
	var hostInfosMu sync.Mutex

	snmpArpTable := make(map[string]string)
	var snmpArpMu sync.Mutex

	var aliveIPs []string
	for ip := range pingAliveIPs {
		aliveIPs = append(aliveIPs, ip)
	}
	// ログやデバッグ時の再現性のため、処理順をIPアドレス順に安定化する
	sort.Strings(aliveIPs)

	// SNMP query for ping-alive hosts
	var wgSNMP sync.WaitGroup
	semSNMP := make(chan bool, 5) // SNMP query concurrency
	for _, ipStr := range aliveIPs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wgSNMP.Add(1)
		semSNMP <- true

		go func(ip string) {
			defer func() {
				<-semSNMP
				wgSNMP.Done()
			}()

			var sysName, sysDesc, snmpMac string
			var snmpArp map[string]string
			var neighbors []LLDPNeighbor
			var snmpPorts []int
			snmpActive := false

			for _, snmpCfg := range cfg.SnmpConfigs {
				sysName, sysDesc, snmpMac, snmpArp, neighbors, snmpPorts = QuerySNMP(ip, snmpCfg.Community, snmpCfg.Mode, snmpCfg.User, snmpCfg.Password, cfg.Timeout, cfg.Retry)
				if sysName != "" || len(neighbors) > 0 || snmpMac != "" {
					snmpActive = true
					if snmpArp != nil && len(snmpArp) > 0 {
						snmpArpMu.Lock()
						for k, v := range snmpArp {
							snmpArpTable[k] = v
						}
						snmpArpMu.Unlock()
					}
					break
				}
			}

			hostInfosMu.Lock()
			hostInfos = append(hostInfos, &HostInfo{
				IP:            ip,
				MAC:           snmpMac,
				SysName:       sysName,
				SysDesc:       sysDesc,
				SNMPActive:    snmpActive,
				SNMPPorts:     snmpPorts,
				LLDPNeighbors: neighbors,
			})
			hostInfosMu.Unlock()
		}(ipStr)
	}
	wgSNMP.Wait()

	progressCallback(50, "Analyzing ARP tables for unpingable hosts...")

	targetIPSet := make(map[string]bool)
	for _, ip := range ips {
		targetIPSet[ip] = true
	}

	detectedIPSet := make(map[string]bool)
	hostInfosMu.Lock()
	for _, host := range hostInfos {
		detectedIPSet[host.IP] = true
	}
	hostInfosMu.Unlock()

	var arpDetectedIPs []string

	// Check local ARP table
	for ip := range localArpTable {
		if targetIPSet[ip] && !detectedIPSet[ip] {
			arpDetectedIPs = append(arpDetectedIPs, ip)
			detectedIPSet[ip] = true

			// ARPで検知された時点で即座にマップに追加（通知）
			if onDetectCallback != nil {
				mac := localArpTable[ip]
				vendor := datastore.FindVendor(mac)
				onDetectCallback(&ScanResult{
					IP:     ip,
					MAC:    mac,
					Vendor: vendor,
				})
			}
		}
	}

	// Check SNMP collected ARP table
	snmpArpMu.Lock()
	for ip := range snmpArpTable {
		if targetIPSet[ip] && !detectedIPSet[ip] {
			arpDetectedIPs = append(arpDetectedIPs, ip)
			detectedIPSet[ip] = true

			// ARPで検知された時点で即座にマップに追加（通知）
			if onDetectCallback != nil {
				mac := snmpArpTable[ip]
				vendor := datastore.FindVendor(mac)
				onDetectCallback(&ScanResult{
					IP:     ip,
					MAC:    mac,
					Vendor: vendor,
				})
			}
		}
	}
	snmpArpMu.Unlock()

	// Query SNMP for ARP-detected (unpingable) hosts
	var wgArpSNMP sync.WaitGroup
	semArpSNMP := make(chan bool, 5)
	for _, ipStr := range arpDetectedIPs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wgArpSNMP.Add(1)
		semArpSNMP <- true

		go func(ip string) {
			defer func() {
				<-semArpSNMP
				wgArpSNMP.Done()
			}()

			var sysName, sysDesc, snmpMac string
			var snmpArp map[string]string
			var neighbors []LLDPNeighbor
			var snmpPorts []int
			snmpActive := false

			for _, snmpCfg := range cfg.SnmpConfigs {
				sysName, sysDesc, snmpMac, snmpArp, neighbors, snmpPorts = QuerySNMP(ip, snmpCfg.Community, snmpCfg.Mode, snmpCfg.User, snmpCfg.Password, cfg.Timeout, cfg.Retry)
				if sysName != "" || len(neighbors) > 0 || snmpMac != "" {
					snmpActive = true
					if snmpArp != nil && len(snmpArp) > 0 {
						snmpArpMu.Lock()
						for k, v := range snmpArp {
							snmpArpTable[k] = v
						}
						snmpArpMu.Unlock()
					}
					break
				}
			}

			hostInfosMu.Lock()
			hostInfos = append(hostInfos, &HostInfo{
				IP:            ip,
				MAC:           snmpMac,
				SysName:       sysName,
				SysDesc:       sysDesc,
				SNMPActive:    snmpActive,
				SNMPPorts:     snmpPorts,
				LLDPNeighbors: neighbors,
			})
			hostInfosMu.Unlock()
		}(ipStr)
	}
	wgArpSNMP.Wait()

	progressCallback(60, "Scanning ports and retrieving banner info...")

	portsToScan := []int{21, 22, 23, 25, 80, 110, 143, 161, 443, 3306, 3389, 5432, 8080, 9100}
	// keyPorts は "off" モード時にバナー取得（TCP接続）で開放を検証するポート一覧。
	// 161 (SNMP) はUDPのため TCP接続は常に失敗するが、バナー検証フェーズで
	// 除外されるため openPorts には残らない。これは意図的な動作である。
	keyPorts := []int{21, 22, 23, 25, 80, 110, 143, 161, 443, 8080}

	var results []*ScanResult
	var resultsMu sync.Mutex

	concurrencyPort := 10
	if cfg.PortScanMode == "off" || cfg.PortScanMode == "safe" {
		concurrencyPort = 3
	}

	semPort := make(chan bool, concurrencyPort)
	var wgPort sync.WaitGroup

	hostInfosMu.Lock()
	totalHosts := len(hostInfos)
	hostsToProcess := make([]*HostInfo, len(hostInfos))
	copy(hostsToProcess, hostInfos)
	hostInfosMu.Unlock()

	for idx, host := range hostsToProcess {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wgPort.Add(1)
		semPort <- true

		go func(index int, h *HostInfo) {
			defer func() {
				<-semPort
				wgPort.Done()
			}()

			timeout := time.Duration(cfg.Timeout) * time.Second
			var openPorts []int

			if h.SNMPActive && len(h.SNMPPorts) > 0 {
				// (1) SNMPに対応しているデバイス：SNMPから取得したポートを利用
				openPorts = h.SNMPPorts
			} else {
				// SNMP非対応
				switch cfg.PortScanMode {
				case "safe":
					// (2) 安全性重視（低速）：スキャン同時実行数を抑えてゆっくりスキャン
					openPorts = scanPortsSlow(h.IP, portsToScan, timeout)
				case "fast":
					// (3) 高速：従来のパラメータでスキャン
					openPorts = ScanPorts(h.IP, portsToScan, timeout)
				default: // "off"
					// (4) OFF：主要ポートを検証リストに加える
					openPorts = keyPorts
				}
			}

			// (5) 検証すべきポートリストに接続して情報を取得 (HTTP, Banner)
			banners := grabBannersAndHTTPInfo(h.IP, openPorts, timeout)

			// もしPortScanModeが"off"だった場合は、バナーが取れた（実際に開いていた）ポートのみをopenPortsとして残す
			if cfg.PortScanMode == "off" && !h.SNMPActive {
				var verifiedPorts []int
				for _, p := range openPorts {
					pStr := strconv.Itoa(p)
					if _, exists := banners[pStr]; exists {
						verifiedPorts = append(verifiedPorts, p)
					}
				}
				openPorts = verifiedPorts
			}

			// MACアドレスの補正
			mac := h.MAC
			if mac == "" {
				mac = localArpTable[h.IP]
			}
			if mac == "" {
				snmpArpMu.Lock()
				mac = snmpArpTable[h.IP]
				snmpArpMu.Unlock()
			}
			vendor := datastore.FindVendor(mac)

			// DNS逆引き
			sysName := h.SysName
			if sysName == "" {
				names, err := net.LookupAddr(h.IP)
				if err == nil && len(names) > 0 {
					sysName = strings.TrimSuffix(names[0], ".")
				}
			}

			res := &ScanResult{
				IP:            h.IP,
				MAC:           mac,
				Vendor:        vendor,
				SysName:       sysName,
				SysDesc:       h.SysDesc,
				OpenPorts:     openPorts,
				LLDPNeighbors: h.LLDPNeighbors,
				Banners:       banners,
			}

			resultsMu.Lock()
			results = append(results, res)
			pct := 60 + int(float64(index+1)/float64(totalHosts)*38.0)
			progressCallback(pct, fmt.Sprintf("Finished details for %s", h.IP))
			if onDetectCallback != nil {
				onDetectCallback(res)
			}
			resultsMu.Unlock()

		}(idx, host)
	}
	wgPort.Wait()

	progressCallback(100, fmt.Sprintf("Scan completed: %d active devices identified", len(results)))
	return results, nil
}

func scanPortsSlow(ip string, ports []int, timeout time.Duration) []int {
	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan bool, 2) // 同時スキャン数を2ポートに制限

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
			// 安全性重視のため、ポートスキャンの合間に適度なウェイト（例: 100ms）を入れる
			time.Sleep(100 * time.Millisecond)
		}(port)
	}
	wg.Wait()
	return openPorts
}

var scriptTagRegex = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
var styleTagRegex = regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
var htmlTagRegex = regexp.MustCompile(`(?i)<[^>]*>`)

var spaceRegex = regexp.MustCompile(`\s+`)

func stripHTMLTags(s string) string {
	s = scriptTagRegex.ReplaceAllString(s, " ")
	s = styleTagRegex.ReplaceAllString(s, " ")
	s = htmlTagRegex.ReplaceAllString(s, " ")
	s = spaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func grabHTTPInfo(ip string, port int, timeout time.Duration) string {
	scheme := "http"
	if port == 443 {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d", scheme, ip, port)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	limitReader := io.LimitReader(resp.Body, 16*1024)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return ""
	}

	title := ""
	titleRegex := regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	if matches := titleRegex.FindSubmatch(bodyBytes); len(matches) > 1 {
		title = strings.TrimSpace(string(matches[1]))
	}

	serverHeader := resp.Header.Get("Server")
	plainText := stripHTMLTags(string(bodyBytes))
	if len(plainText) > 400 {
		plainText = plainText[:400] + "..."
	}

	var infoParts []string
	if title != "" {
		infoParts = append(infoParts, "Title: "+title)
	}
	if serverHeader != "" {
		infoParts = append(infoParts, "Server: "+serverHeader)
	}
	if plainText != "" {
		infoParts = append(infoParts, "Text: "+plainText)
	}

	return strings.Join(infoParts, " | ")
}

func grabBanner(ip string, port int, timeout time.Duration) string {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(timeout))

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}

	banner := strings.TrimSpace(string(buf[:n]))
	banner = strings.ReplaceAll(banner, "\r", "")
	banner = strings.ReplaceAll(banner, "\n", " ")
	if len(banner) > 200 {
		banner = banner[:200] + "..."
	}
	return banner
}

func grabBannersAndHTTPInfo(ip string, openPorts []int, timeout time.Duration) map[string]string {
	banners := make(map[string]string)
	for _, port := range openPorts {
		var info string
		if port == 80 || port == 443 || port == 8080 {
			info = grabHTTPInfo(ip, port, timeout)
		} else if port == 21 || port == 22 || port == 23 || port == 25 || port == 110 || port == 143 {
			info = grabBanner(ip, port, timeout)
		}
		if info != "" {
			banners[strconv.Itoa(port)] = info
		}
	}
	if len(banners) == 0 {
		return nil
	}
	return banners
}

