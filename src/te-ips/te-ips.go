package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	Ver              = "0.2"
	ApiUrl           = "https://api.thousandeyes.com/agents.json"
	IPList           = "ip"
	SubnetListStrict = "subnet-strict"
	SubnetListLoose  = "subnet-loose"
	CSV              = "csv"
	JSON             = "json"
	XML              = "xml"
	Enterprise = "Enterprise"
	Cloud = "Cloud"
)

var log = new(Log)

type Agent struct {
	AgentID           int      `json:"agentId"`
	AgentName         string   `json:"agentName"`
	AgentType         string   `json:"agentType"`
	Location          string   `json:"location"`
	CountryID         string   `json:"countryId"`
	IPAddresses       []string `json:"ipAddresses"`
	IPv4Addresses     []net.IP
	IPv6Addresses     []net.IP
	IPv4SubnetsStrict []net.IPNet
	IPv6SubnetsStrict []net.IPNet
	IPv4SubnetsLoose  []net.IPNet
	IPv6SubnetsLoose  []net.IPNet
}

func main() {

	// Flags
	version := flag.Bool("v", false, "Prints out version")
	output := flag.String("o", SubnetListStrict, "Output type ("+IPList+", "+SubnetListStrict+", "+SubnetListLoose+", "+CSV+", "+JSON+", "+XML+")")
	user := flag.String("u", "", "ThousandEyes user")
	token := flag.String("t", "", "ThousandEyes user API token")
	i4 := flag.Bool("4", false, "Display only IPv4 addresses")
	i6 := flag.Bool("6", false, "Display only IPv6 addresses")
	ea := flag.Bool("e", false, "Display only Enterprise Agent addresses")
	ca := flag.Bool("c", false, "Display only Cloud Agent addresses")
	flag.Parse()

	if *version == true {
		fmt.Printf("ThousandEyes Agent IP List v%s (%s/%s)\n\n", Ver, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	var ipv4, ipv6 bool
	if *i4 && !*i6 {
		ipv4 = true
		ipv6 = false
	} else if !*i4 && *i6 {
		ipv4 = false
		ipv6 = true
	} else {
		ipv4 = true
		ipv6 = true
	}

	var enterprise, cloud bool
	if *ea && !*ca {
		enterprise = true
		cloud = false
	} else if !*ea && *ca {
		enterprise = false
		cloud = true
	} else {
		enterprise = true
		cloud = true
	}

	if !validateEmail(*user) {
		log.Error("'%s' is not a valid ThousandEyes user.", *user)
		os.Exit(0)
	}

	if !validateToken(*token) {
		log.Error("'%s' is not a valid ThousandEyes user API token. Find your token at https://app.thousandeyes.com/settings/account/?section=profile", *token)
		os.Exit(0)
	}

	agents, _ := fetchAgents(*user, *token, enterprise, cloud, ipv4, ipv6)

	if strings.ToLower(*output) == IPList {
		outputIPList(agents)
	} else if strings.ToLower(*output) == SubnetListStrict {
		outputSubnetListStrict(agents)
	} else if strings.ToLower(*output) == SubnetListLoose {
		outputSubnetListLoose(agents)
	} else if strings.ToLower(*output) == CSV {
		outputCSV(agents)
	} else if strings.ToLower(*output) == JSON {
		outputJSON(agents)
	} else if strings.ToLower(*output) == XML {
		outputXML(agents)
	} else {
		log.Error("Output type '%s' not supported. Supported output types: %s, %s, %s, %s", *output, IPList, CSV, JSON, XML)
		os.Exit(0)
	}

}

func validateEmail(email string) bool {
	Re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return Re.MatchString(email)
}

func validateToken(token string) bool {
	Re := regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)
	return Re.MatchString(token)
}

func fetchAgents(user string, token string, enterprise bool, cloud bool, ipv4 bool, ipv6 bool) ([]Agent, error) {

	type Agents struct {
		Agents []Agent `json:"agents"`
	}

	var agents Agents

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	var netClient = &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}
	request, _ := http.NewRequest("GET", ApiUrl, nil)
	request.SetBasicAuth(user, token)
	response, _ := netClient.Do(request)

	if response.StatusCode == http.StatusOK {
		// yupepeeee
	} else if response.StatusCode == http.StatusTooManyRequests {
		// handle it
	} else {
		log.Error("TE API HTTP error: %s (%s)", response.Status, response.StatusCode)
	}

	err := json.NewDecoder(response.Body).Decode(&agents)
	if err != nil {
		return []Agent{}, err
	}

	if !enterprise || !cloud {
		for i := len(agents.Agents) - 1; i >= 0; i-- {
	    agent := agents.Agents[i]
	    // Condition to decide if current element has to be deleted:
	    if enterprise && agent.AgentType == Enterprise {
	      // Keep it
	    }	else if cloud && agent.AgentType == Cloud {
	      // Keep it
	    } else {
				agents.Agents = append(agents.Agents[:i], agents.Agents[i+1:]...)
			}
		}
	}

	for i, agent := range agents.Agents {
		if len(agent.IPAddresses) > 0 {
			for _, ip := range agent.IPAddresses {
				if ipv6 && strings.Contains(ip, ":") {
					agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
				} else if ipv4 && strings.Contains(ip, ".") {
					agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
				}
			}
			agents.Agents[i].IPAddresses = []string{}
		}
	}

	return agents.Agents, nil
}

func outputIPList(agents []Agent) {

	ips := sortAgentIPs(agents)

	for _, ip := range ips {
		fmt.Printf("%s\n", ip.String())
	}

}

func outputSubnetListStrict(agents []Agent) {

	ips := sortAgentIPs(agents)
	ipNets := ipsToSubnetsStrict(ips)

	for _, ipNet := range ipNets {
		s, t := ipNet.Mask.Size()
		if s == t {
			fmt.Printf("%s\n", ipNet.IP.String())
		} else {
			fmt.Printf("%s\n", ipNet.String())
		}
	}

}

func outputSubnetListLoose(agents []Agent) {

	ips := sortAgentIPs(agents)
	ipNets := ipsToSubnetsLoose(ips)

	for _, ipNet := range ipNets {
		s, t := ipNet.Mask.Size()
		if s == t {
			fmt.Printf("%s\n", ipNet.IP.String())
		} else {
			fmt.Printf("%s\n", ipNet.String())
		}
	}

}

func outputCSV(agents []Agent) {

	fmt.Printf("Agent ID;Agent Name;Agent Type;Location;Country;IPv4 Addresses;IPv4 Subnets (Strict);IPv4 Subnets (Loose);IPv6 Addresses;IPv6 Subnets (Strict);IPv6 Subnets (Loose)\n")

	agents = addSubnetsToAgents(agents)

	for _, agent := range agents {
		fmt.Printf("%s;%s;%s;%s;%s;", strconv.Itoa(agent.AgentID), agent.AgentName, agent.AgentType, agent.Location, agent.CountryID)

		ipStr := ""
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				ipStr = ipStr + ip.String() + ","
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)
		ipStr = ""
		if len(agent.IPv4SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv4SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + ","
				} else {
					ipStr = ipStr + ipNet.String() + ","
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)
		ipStr = ""
		if len(agent.IPv4SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv4SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + ","
				} else {
					ipStr = ipStr + ipNet.String() + ","
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)
		ipStr = ""
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				ipStr = ipStr + ip.String() + ","
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)
		ipStr = ""
		if len(agent.IPv6SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv6SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + ","
				} else {
					ipStr = ipStr + ipNet.String() + ","
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)
		ipStr = ""
		if len(agent.IPv6SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv6SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + ","
				} else {
					ipStr = ipStr + ipNet.String() + ","
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("%s;", ipStr)

		fmt.Printf("\n")
	}

}

func outputJSON(agents []Agent) {

	type OutputAgent struct {
		AgentID           int      `json:"agentId"`
		AgentName         string   `json:"agentName"`
		AgentType         string   `json:"agentType"`
		Location          string   `json:"location"`
		CountryID         string   `json:"countryId"`
		IPv4Addresses     []string `json:"ipv4Addresses,omitempty"`
		IPv6Addresses     []string `json:"ipv6Addresses,omitempty"`
		IPv4SubnetsStrict []string `json:"ipv4SubnetsStrict,omitempty"`
		IPv6SubnetsStrict []string `json:"ipv6SubnetsStrict,omitempty"`
		IPv4SubnetsLoose  []string `json:"ipv4SubnetsLoose,omitempty"`
		IPv6SubnetsLoose  []string `json:"ipv6SubnetsLoose,omitempty"`
	}

	outputAgents := []OutputAgent{}
	agents = addSubnetsToAgents(agents)

	for _, agent := range agents {
		outputAgent := OutputAgent{AgentID: agent.AgentID, AgentName: agent.AgentName, AgentType: agent.AgentType, Location: agent.Location, CountryID: agent.CountryID}
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				outputAgent.IPv4Addresses = append(outputAgent.IPv4Addresses, ip.String())
			}
		}
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				outputAgent.IPv6Addresses = append(outputAgent.IPv6Addresses, ip.String())
			}
		}
		if len(agent.IPv4SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv4SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv4SubnetsStrict = append(outputAgent.IPv4SubnetsStrict, ipNet.IP.String())
				} else {
					outputAgent.IPv4SubnetsStrict = append(outputAgent.IPv4SubnetsStrict, ipNet.String())
				}
			}
		}
		if len(agent.IPv6SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv6SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv6SubnetsStrict = append(outputAgent.IPv6SubnetsStrict, ipNet.IP.String())
				} else {
					outputAgent.IPv6SubnetsStrict = append(outputAgent.IPv6SubnetsStrict, ipNet.String())
				}
			}
		}
		if len(agent.IPv4SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv4SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv4SubnetsLoose = append(outputAgent.IPv4SubnetsLoose, ipNet.IP.String())
				} else {
					outputAgent.IPv4SubnetsLoose = append(outputAgent.IPv4SubnetsLoose, ipNet.String())
				}
			}
		}
		if len(agent.IPv6SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv6SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv6SubnetsLoose = append(outputAgent.IPv6SubnetsLoose, ipNet.IP.String())
				} else {
					outputAgent.IPv6SubnetsLoose = append(outputAgent.IPv6SubnetsLoose, ipNet.String())
				}
			}
		}
		outputAgents = append(outputAgents, outputAgent)
	}

	j, _ := json.MarshalIndent(outputAgents, "", "  ")

	fmt.Printf("%s", string(j))
}

func outputXML(agents []Agent) {

	type OutputAgent struct {
		AgentID           int      `xml:"agentId"`
		AgentName         string   `xml:"agentName"`
		AgentType         string   `xml:"agentType"`
		Location          string   `xml:"location"`
		CountryID         string   `xml:"countryId"`
		IPv4Addresses     []string `xml:"ipv4Addresses,omitempty"`
		IPv6Addresses     []string `xml:"ipv6Addresses,omitempty"`
		IPv4SubnetsStrict []string `xml:"ipv4SubnetsStrict,omitempty"`
		IPv6SubnetsStrict []string `xml:"ipv6SubnetsStrict,omitempty"`
		IPv4SubnetsLoose  []string `xml:"ipv4SubnetsLoose,omitempty"`
		IPv6SubnetsLoose  []string `xml:"ipv6SubnetsLoose,omitempty"`
	}

	outputAgents := []OutputAgent{}
	agents = addSubnetsToAgents(agents)

	for _, agent := range agents {
		outputAgent := OutputAgent{AgentID: agent.AgentID, AgentName: agent.AgentName, AgentType: agent.AgentType, Location: agent.Location, CountryID: agent.CountryID}
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				outputAgent.IPv4Addresses = append(outputAgent.IPv4Addresses, ip.String())
			}
		}
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				outputAgent.IPv6Addresses = append(outputAgent.IPv6Addresses, ip.String())
			}
		}
		if len(agent.IPv4SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv4SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv4SubnetsStrict = append(outputAgent.IPv4SubnetsStrict, ipNet.IP.String())
				} else {
					outputAgent.IPv4SubnetsStrict = append(outputAgent.IPv4SubnetsStrict, ipNet.String())
				}
			}
		}
		if len(agent.IPv6SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv6SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv6SubnetsStrict = append(outputAgent.IPv6SubnetsStrict, ipNet.IP.String())
				} else {
					outputAgent.IPv6SubnetsStrict = append(outputAgent.IPv6SubnetsStrict, ipNet.String())
				}
			}
		}
		if len(agent.IPv4SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv4SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv4SubnetsLoose = append(outputAgent.IPv4SubnetsLoose, ipNet.IP.String())
				} else {
					outputAgent.IPv4SubnetsLoose = append(outputAgent.IPv4SubnetsLoose, ipNet.String())
				}
			}
		}
		if len(agent.IPv6SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv6SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					outputAgent.IPv6SubnetsLoose = append(outputAgent.IPv6SubnetsLoose, ipNet.IP.String())
				} else {
					outputAgent.IPv6SubnetsLoose = append(outputAgent.IPv6SubnetsLoose, ipNet.String())
				}
			}
		}
		outputAgents = append(outputAgents, outputAgent)
	}

	x, _ := xml.MarshalIndent(outputAgents, "", "  ")

	fmt.Printf("%s", string(x))
}

// Sort agent IPs, IPv4 first, IPv6 following
func sortAgentIPs(agents []Agent) []net.IP {

	ipv4IPs := []net.IP{}
	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				ipv4IPs = append(ipv4IPs, ip)
			}
		}
	}
	ipv4IPs = sortIPs(ipv4IPs)

	ipv6IPs := []net.IP{}
	for _, agent := range agents {
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				ipv6IPs = append(ipv6IPs, ip)
			}
		}
	}
	ipv6IPs = sortIPs(ipv6IPs)

	return append(ipv4IPs, ipv6IPs...)

}

func addSubnetsToAgents(agents []Agent) []Agent {

	for i, agent := range agents {
		if len(agent.IPv4Addresses) > 0 {
			agents[i].IPv4SubnetsStrict = ipsToSubnetsStrict(agent.IPv4Addresses)
			agents[i].IPv4SubnetsLoose = ipsToSubnetsLoose(agent.IPv4Addresses)
		}
		if len(agent.IPv6Addresses) > 0 {
			agents[i].IPv6SubnetsStrict = ipsToSubnetsStrict(agent.IPv6Addresses)
			agents[i].IPv6SubnetsLoose = ipsToSubnetsLoose(agent.IPv6Addresses)
		}
	}

	return agents

}

// Sort the list of IPs numerically
func sortIPs(ips []net.IP) []net.IP {

	sort.Stable(IPSlice(ips))

	uniqueIps := []net.IP{}
	for i, ip := range ips {
		if len(ips) > i+1 && bytes.Compare(ip, ips[i+1]) == 0 {

		} else {
			uniqueIps = append(uniqueIps, ip)
		}
	}

	return uniqueIps

}

// Transform a list of IPs to a list of subnets that exactly match the list of
// IPs
func ipsToSubnetsStrict(ips []net.IP) []net.IPNet {

	ipNets := []net.IPNet{}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		if ip.To4() != nil {
			// IPv4
			hostLen := 32
			minParentLen := 24
			for parentLen := minParentLen; parentLen <= hostLen; parentLen++ {
				parentMask := net.CIDRMask(parentLen, hostLen)
				parentNetId := ip.Mask(parentMask)
				parentSubnet := net.IPNet{parentNetId, parentMask}
				maxHostsInSubnet := int(math.Pow(2, float64(hostLen-parentLen)))
				parentSubnetForAllHosts := true
				for n := 1; n < maxHostsInSubnet; n++ {
					if len(ips) > i+n && parentSubnet.Contains(ips[i+n]) {
						// Next N IP address belongs to the same subnet
					} else {
						parentSubnetForAllHosts = false
						break
					}
				}
				if parentSubnetForAllHosts == true {
					ipNets = append(ipNets, parentSubnet)
					iAlreadyCovered = i + maxHostsInSubnet - 1
					break
				}
			}
		} else {
			// IPv6
			// Not much we can do here, don't want to go /64 for strict mode, and with
			// autoconfigured IP addresses there is no point summarizing prefixes smaller
			// than /64
			parentMask := net.CIDRMask(128, 128)
			parentSubnet := net.IPNet{ip, parentMask}
			ipNets = append(ipNets, parentSubnet)
		}
	}

	return ipNets

}

// Transform a list of IPs to a minimal list of /24 or longer subnets that
// covers all the input IPs but also some of the IPs that are not on the input
// list
func ipsToSubnetsLoose(ips []net.IP) []net.IPNet {

	ipNets := []net.IPNet{}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		if ip.To4() != nil {
			// IPv4
			hostLen := 32
			minParentLen := 24
			previousSubnetHosts := 0
			previousSubnet := net.IPNet{}
			for parentLen := minParentLen; parentLen <= hostLen; parentLen++ {
				parentMask := net.CIDRMask(parentLen, hostLen)
				parentNetId := ip.Mask(parentMask)
				parentSubnet := net.IPNet{parentNetId, parentMask}
				maxHostsInSubnet := int(math.Pow(2, float64(hostLen-parentLen)))
				hostsInSubnet := 1
				for n := 1; n < maxHostsInSubnet; n++ {
					if len(ips) > i+n && parentSubnet.Contains(ips[i+n]) {
						// Next N IP address belongs to the same subnet
						hostsInSubnet++
					} else {
						break
					}
				}

				if hostsInSubnet >= previousSubnetHosts {
					// This subnet covers all hosts than a wider subnet, so it is a
					// better choice
					if parentLen == hostLen {
						ipNets = append(ipNets, parentSubnet)
						break
					} else {
						previousSubnetHosts = hostsInSubnet
						previousSubnet = parentSubnet
					}
				} else {
					// Previous subnet covered more, lets use it
					ipNets = append(ipNets, previousSubnet)
					iAlreadyCovered = i + previousSubnetHosts - 1
					break
				}

				previousSubnetHosts = hostsInSubnet
			}
		} else {
			// IPv6
			// Simply /64 for now
			hostLen := 128
			parentLen := 64

			parentMask := net.CIDRMask(parentLen, hostLen)
			parentNetId := ip.Mask(parentMask)
			parentSubnet := net.IPNet{parentNetId, parentMask}
			hostsInSubnet := 1
			for n := 1; n < 1000; n++ {
				if len(ips) > i+n && parentSubnet.Contains(ips[i+n]) {
					// Next N IP address belongs to the same subnet
					hostsInSubnet++
				} else {
					break
				}
			}

			ipNets = append(ipNets, parentSubnet)
			if hostsInSubnet != 1 {
				iAlreadyCovered = i + hostsInSubnet - 1
			}

		}
	}

	return ipNets

}

// Logger
type Log struct{}

func (log *Log) Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, time.Now().Format("2006-01-02 15:04:05 ")+" ERROR  "+format+"\n", a...)
}

// IPSlice attaches the methods of Sort Interface to []net.IP, sorting in increasing order.
type IPSlice []net.IP

func (p IPSlice) Len() int { return len(p) }
func (p IPSlice) Less(i, j int) bool {
	c := bytes.Compare(p[i], p[j])
	if c == -1 {
		return true
	} else {
		return false
	}
}
func (p IPSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
