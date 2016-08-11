package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	ver              = "0.1"
	apiURL           = "https://api.thousandeyes.com/agents.json"
	IPList           = "ip"
	SubnetListStrict = "subnet-strict"
	SubnetListLoose  = "subnet-loose"
	CSV              = "csv"
	JSON             = "json"
	XML              = "xml"
)

var log = new(Log)

type Agent struct {
	AgentId       int      `json:"agentId"`
	AgentName     string   `json:"agentName"`
	AgentType     string   `json:"agentType"`
	Location      string   `json:"location"`
	CountryId     string   `json:"countryId"`
	IPAddresses   []string `json:"ipAddresses"`
	IPv4Addresses []net.IP
	IPv6Addresses []net.IP
}

func main() {

	// Flags
	version := flag.Bool("v", false, "Prints out version")
	output := flag.String("o", SubnetListStrict, "Output type ("+IPList+", "+SubnetListStrict+", "+SubnetListLoose+")")
	user := flag.String("u", "", "ThousandEyes user")
	token := flag.String("t", "", "ThousandEyes user API token")
	i4 := flag.Bool("4", false, "Display only IPv4 addresses")
	i6 := flag.Bool("6", false, "Display only IPv6 addresses")
	flag.Parse()

	if *version == true {
		fmt.Printf("ThousandEyes Agent IP List v%s (%s/%s)\n\n", ver, runtime.GOOS, runtime.GOARCH)
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

	if !validateEmail(*user) {
		log.Error("'%s' is not a valid ThousandEyes user.", *user)
		os.Exit(0)
	}

	if !validateToken(*token) {
		log.Error("'%s' is not a valid ThousandEyes user API token. Find your token at https://app.thousandeyes.com/settings/account/?section=profile", *token)
		os.Exit(0)
	}

	agents, _ := fetchAgents(*user, *token, ipv4, ipv6)

	if strings.ToLower(*output) == IPList {
		outputIPList(agents)
	} else if strings.ToLower(*output) == SubnetListStrict {
		outputSubnetListStrict(agents)
	} else if strings.ToLower(*output) == SubnetListLoose {
		outputSubnetListLoose(agents)
	} else if strings.ToLower(*output) == CSV {
		//outputCSV(agents)
	} else if strings.ToLower(*output) == JSON {
		//outputJSON(agents)
	} else if strings.ToLower(*output) == XML {
		//outputXML(agents)
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

func fetchAgents(user string, token string, ipv4 bool, ipv6 bool) ([]Agent, error) {

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
	request, _ := http.NewRequest("GET", apiURL, nil)
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

	for i, agent := range agents.Agents {
		if len(agent.IPAddresses) > 0 {
			for _, ip := range agent.IPAddresses {
				if ipv6 && strings.Contains(ip, ":") {
					agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
				} else if ipv4 && strings.Contains(ip, ".") {
					agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
				} else {
					log.Error("Cannot recognize the IP address string '%s'.", ip)
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

	iAlreadyCovered := 0
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
					if parentLen == hostLen {
						fmt.Printf("%s\n", ip.String())
					} else {
						fmt.Printf("%s\n", parentSubnet.String())
					}
					iAlreadyCovered = i + maxHostsInSubnet - 1
					break
				}
			}
		} else {
			// IPv6
			// Not much we can do here, don't want to go /64 for strict mode, and with
			// autoconfigured IP addresses there is no point summarizing prefixes smaller
			// than /64
			fmt.Printf("%s\n", ip.String())
		}
	}

}

func outputSubnetListLoose(agents []Agent) {

	ips := sortAgentIPs(agents)

	iAlreadyCovered := 0
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
						fmt.Printf("%s\n", ip.String())
						break
					} else {
						previousSubnetHosts = hostsInSubnet
						previousSubnet = parentSubnet
					}
				} else {
					// Previous subnet covered more, lets use it
					fmt.Printf("%s\n", previousSubnet.String())
					iAlreadyCovered = i + previousSubnetHosts - 1
					break
				}

				previousSubnetHosts = hostsInSubnet
			}
		} else {
			// IPv6
			// Simply /64 for Now
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

			if hostsInSubnet == 1 {
				fmt.Printf("%s\n", ip.String())
			} else {
				fmt.Printf("%s\n", parentSubnet.String())
				iAlreadyCovered = i + hostsInSubnet - 1
			}

		}
	}

}

func sortAgentIPs(agents []Agent) []net.IP {

	ipv4IPs := []net.IP{}
	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				ipv4IPs = append(ipv4IPs, ip)
			}
		}
	}
	sort.Stable(IPSlice(ipv4IPs))

	ipv6IPs := []net.IP{}
	for _, agent := range agents {
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				ipv6IPs = append(ipv6IPs, ip)
			}
		}
	}
	sort.Stable(IPSlice(ipv6IPs))

	ips := append(ipv4IPs, ipv6IPs...)

	uniqueIps := []net.IP{}
	for i, ip := range ips {
		if len(ips) > i+1 && bytes.Compare(ip, ips[i+1]) == 0 {

		} else {
			uniqueIps = append(uniqueIps, ip)
		}
	}

	return uniqueIps

}

// Logger
type Log struct{}

func (log *Log) Error(format string, a ...interface{}) {
	fmt.Printf(os.Stderr, time.Now().Format("2006-01-02 15:04:05 ")+" ERROR  "+format+"\n", a...)
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
