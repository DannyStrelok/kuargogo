package tplink

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network"
	"golang.org/x/net/publicsuffix"
)

// SG108E represents a TP-Link Easy Smart Switch via HTTP
type SG108E struct {
	IP       string
	User     string
	Password string
	client   *http.Client
	baseURL  string
}

func NewSG108E(ip, user, password string) network.SwitchController {
	// Create a cookie jar to handle session
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	client := &http.Client{
		Timeout: 5 * time.Second,
		Jar:     jar,
	}

	return &SG108E{
		IP:       ip,
		User:     user,
		Password: password,
		client:   client,
		baseURL:  fmt.Sprintf("http://%s", ip),
	}
}

func (s *SG108E) Connect() error {
	// TP-Link V2/V3 usually requires a POST to /Logon.cgi or similar
	// We'll try the standard V2/V3 login flow

	loginURL := fmt.Sprintf("%s/logon.cgi", s.baseURL)

	// Payload for many TP-Link Easy Smart switches
	data := url.Values{}
	data.Set("username", s.User)
	data.Set("password", s.Password)
	data.Set("logon", "Login")

	resp, err := s.client.PostForm(loginURL, data)
	if err != nil {
		return fmt.Errorf("failed to connect to switch: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	// Some models return a specific cookie or token in body script
	// But mostly the CookieJar handles the JSESSIONID
	return nil
}

func (s *SG108E) Close() error {
	// Best effort logout
	// s.client.Get(fmt.Sprintf("%s/Logout.cgi", s.baseURL))
	return nil
}

func (s *SG108E) GetStatus() (*network.SwitchStatus, error) {
	url := fmt.Sprintf("%s/PortStatisticsRpm.htm", s.baseURL)

	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch status page: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request failed: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	// In TP-Link Easy Smart switches, port data is usually embedded in a JS array.
	// Example: var statisInfo = new Array(
	// 1, 1, 6, 178234, 0, 56345, 0,
	// 2, 0, 0, 0, 0, 0, 0,
	// ...
	// Where: PortID, Status (1=Up, 0=Down), SpeedLink(5=100M, 6=1000M, etc.)

	// For maximum compatibility across v1/v2/v3/v6 firmwares, we look for State (Up/Down) and Speed (1000MF) strings if exposed in HTML table, or parse the most common JS array format.
	// Many modern TP-Links (V3+) use: `state:[1,0,1...]` or `link_status:[5,0,6...]`

	// Let's use a robust approach for the most common firmware (V2/V3):
	// var statisInfo = new Array(
	// portId, status (1 or 0), speed_index, ...
	var ports []network.PortStatus

	// Match:  `1, 1, 6,` -> Port 1, Up, 1000Mbps
	// Match:  `2, 0, 0,` -> Port 2, Down, Auto
	// Using regex to find patterns of 7 or 8 numbers separated by commas inside Array.
	// Since port num is sequentially 1 to 8 (or 16), we can anchor on `\b(\d+),\s*([01]),\s*(\d+),`

	// Looking for (Port), (Status 0/1), (Speed 0-6), (RxGood), (RxBad), (TxGood), (TxBad)
	reJS := regexp.MustCompile(`\n\s*(\d{1,2}),\s*([01]),\s*(\d{1,2}),\s*(\d+),\s*(\d+),\s*(\d+),\s*(\d+),?`)
	matches := reJS.FindAllStringSubmatch(body, -1)

	if len(matches) > 0 {
		for _, match := range matches {
			portID := fmt.Sprintf("port_%s", match[1])
			isUp := match[2] == "1"

			speedIdx := match[3]
			var speed network.Speed

			if isUp {
				switch speedIdx {
				case "2":
					speed = network.Speed10M
				case "3":
					speed = network.Speed10M
				case "4":
					speed = network.Speed100M
				case "5":
					speed = network.Speed100M
				case "6":
					speed = network.Speed1G
				default:
					speed = network.SpeedUnknown
				}
			} else {
				speed = network.SpeedDown
			}

			// bytes/packets roughly
			rxGood := match[4]
			txGood := match[6]
			// We can convert these to uint64 if needed, but for now we focus on status

			ports = append(ports, network.PortStatus{
				ID:       portID,
				Name:     fmt.Sprintf("Port %s", match[1]),
				IsUp:     isUp,
				Speed:    speed,
				BytesIn:  parseUint(rxGood),
				BytesOut: parseUint(txGood),
			})
		}
	} else {
		// Fallback for HTML based firmware
		// Look for <td>1</td><td>Up</td><td>1000MF</td>
		reHTML := regexp.MustCompile(`>(\d{1,2})<.*?>(Up|Down)<.*?>(1000MF|100MF|10MF|Auto|--)<`)
		htmlMatches := reHTML.FindAllStringSubmatch(body, -1)

		for _, match := range htmlMatches {
			portNum := match[1]
			statusStr := match[2]
			speedStr := match[3]

			isUp := statusStr == "Up"
			speed := network.SpeedDown

			if isUp {
				if strings.Contains(speedStr, "1000") {
					speed = network.Speed1G
				} else if strings.Contains(speedStr, "100") {
					speed = network.Speed100M
				} else if strings.Contains(speedStr, "10") {
					speed = network.Speed10M
				} else {
					speed = network.SpeedUnknown
				}
			}

			ports = append(ports, network.PortStatus{
				ID:    fmt.Sprintf("port_%s", portNum),
				Name:  fmt.Sprintf("Port %s", portNum),
				IsUp:  isUp,
				Speed: speed,
			})
		}
	}

	return &network.SwitchStatus{
		Hostname: "TP-Link SG108E",
		IP:       s.IP,
		Model:    "TL-SG108E",
		Ports:    ports,
	}, nil
}

// parseUint helper
func parseUint(s string) uint64 {
	var u uint64
	_, err := fmt.Sscanf(s, "%d", &u)
	if err != nil {
		log.Printf("Warning: failed to parse uint from string %s: %v\n", s, err)
	}
	return u
}

func (s *SG108E) ApplyConfig(layout config.NetworkLayout) error {
	return fmt.Errorf("ApplyConfig: VLAN configuration via HTTP not yet implemented")
}

func (s *SG108E) GetMACTable() (map[string]string, error) {
	// Target URL for SG108E V3 (and likely others)
	url := fmt.Sprintf("%s/AddressTableRpm.htm", s.baseURL)

	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MAC table: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAC table request failed: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	// TP-Link SG108E typically exposes data in a JavaScript array named `macPara` or similar,
	// or in an HTML table.
	// Common format in these embedded devices:
	// new Array("00-11-22-33-44-55", 1, 1, ...)
	// Where 2nd arg is likely Port ID.
	// Or HTML: <td>00-23-24-00-00-03</td>...<td>8</td>

	// We use a robust Regex to find MAC addresses and the nearest subsequent integer (Port).
	// Regex: MAC followed by some chars, then a number (Port)
	// Mac format: XX-XX-XX-XX-XX-XX

	macTable := make(map[string]string)

	// Regex to capture MAC (hyphen separated) and then look for the port number
	// Example finding: "00-E0-4C-68-00-01", 1
	re := regexp.MustCompile(`([0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}).*?,\s*(\d+)`)

	// Note: Allow flexible chars between MAC and Port, but usually it's `", ` in JS arrays or `</td><td>` in HTML.
	// Adjusting regex for JS Array format which is most likely:
	// `... "MAC", PortID, ...`

	matches := re.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		if len(match) == 3 {
			// Normalize MAC to lowercase colon-separated to match kuargogo.yaml standard
			rawMac := match[1]
			portNum := match[2]

			normMac := strings.ReplaceAll(strings.ToLower(rawMac), "-", ":")
			portID := fmt.Sprintf("port_%s", portNum)

			// Store in map: Port -> MAC
			// Note: If multiple MACs on one port, last one wins (Limitation of map[string]string)
			// But for "Inventory Map" of physical wiring, this usually suffices.
			macTable[portID] = normMac
		}
	}

	// Fallback/Validation: If 0 matches, maybe try HTML table regex?
	if len(macTable) == 0 {
		// Try HTML TD pattern: <td>MAC</td> ... <td>PORT</td>
		reHTML := regexp.MustCompile(`>([0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2}-[0-9A-Fa-f]{2})<.*?(\d+)<`)
		matchesHTML := reHTML.FindAllStringSubmatch(body, -1)
		for _, match := range matchesHTML {
			if len(match) == 3 {
				rawMac := match[1]
				portNum := match[2]
				normMac := strings.ReplaceAll(strings.ToLower(rawMac), "-", ":")
				portID := fmt.Sprintf("port_%s", portNum)
				macTable[portID] = normMac
			}
		}
	}

	return macTable, nil
}

func (s *SG108E) Reboot() error {
	return nil
}
