package main

import (
	"bytes"
	"encoding/binary"
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
	Ver               = "0.8"
	ApiUrl            = "https://api.thousandeyes.com/agents.json"
	IPList            = "ip"
	SubnetListStrict  = "subnet-strict"
	SubnetListLoose   = "subnet-loose"
	IPRangeListStrict = "range-strict"
	IPRangeListLoose  = "range-loose"
	IPBlockListStrict = "block-strict"
	IPBlockListLoose  = "block-loose"
	CSV               = "csv"
	JSON              = "json"
	XML               = "xml"
	Enterprise        = "Enterprise"
	EnterpriseCluster = "Enterprise Cluster"
	Cloud             = "Cloud"
	ListCommentChar   = "#"
	ListSeparatorChar = ";"
	CSVSeparatorChar  = ","
)

var log = new(Log)

type Agent struct {
	// Imported from input JSON
	AgentID           int      `json:"agentId"`
	AgentName         string   `json:"agentName"`
	AgentType         string   `json:"agentType"`
	Location          string   `json:"location"`
	CountryID         string   `json:"countryId"`
	IPAddresses       []string `json:"ipAddresses"`
	PublicIPAddresses []string `json:"publicIpAddresses"`
	ClusterMembers    []Agent  `json:"clusterMembers"`
	// Generated
	IPv4Addresses     []net.IP
	IPv6Addresses     []net.IP
	IPv4SubnetsStrict []net.IPNet
	IPv6SubnetsStrict []net.IPNet
	IPv4SubnetsLoose  []net.IPNet
	IPv6SubnetsLoose  []net.IPNet
	IPv4RangesStrict  []IPRange
	IPv6RangesStrict  []IPRange
	IPv4RangesLoose   []IPRange
	IPv6RangesLoose   []IPRange
	IPv4BlocksStrict  []IPBlock
	IPv6BlocksStrict  []IPBlock
	IPv4BlocksLoose   []IPBlock
	IPv6BlocksLoose   []IPBlock
}

func main() {

	// Flags
	version := flag.Bool("v", false, "Prints out version")
	output := flag.String("o", SubnetListStrict, "Output type ("+IPList+", "+SubnetListStrict+", "+SubnetListLoose+", "+IPRangeListStrict+", "+IPRangeListLoose+", "+IPBlockListStrict+", "+IPBlockListLoose+", "+CSV+", "+JSON+", "+XML+")")
	user := flag.String("u", "", "ThousandEyes user")
	token := flag.String("t", "", "ThousandEyes user API token")
	i4 := flag.Bool("4", false, "Display only IPv4 addresses")
	i6 := flag.Bool("6", false, "Display only IPv6 addresses")
	ea := flag.Bool("e", false, "Display only Enterprise Agent addresses")
	ca := flag.Bool("c", false, "Display only Cloud Agent addresses")
	eaPub := flag.Bool("e-public", false, "Display only Enterprise Agent Public IP addresses")
	eaPriv := flag.Bool("e-private", false, "Display only Enterprise Agent Private IP addresses")
	name := flag.Bool("n", false, "Add Agent name as a comment to "+IPList+", "+SubnetListStrict+", "+SubnetListLoose+", "+IPRangeListStrict+", "+IPRangeListLoose+", "+IPBlockListStrict+" and "+IPBlockListLoose+" output types.")
	flag.Parse()

	if *version == true {
		fmt.Printf("\nThousandEyes Agent IP List v%s (%s/%s)\n\n", Ver, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if *user == "" && *token == "" {
		fmt.Printf("\nThousandEyes Agent IP List v%s (%s/%s)\n\n", Ver, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Usage:\n  %s -u <user> -t <user-api-token>\n\nHelp:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Printf("\n")
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
	if (*ea || *eaPub || *eaPriv) && !*ca {
		enterprise = true
	} else if !*ea && !*eaPub && !*eaPriv && *ca {
		cloud = true
	} else {
		enterprise = true
		cloud = true
	}

	var enterprisePublic, enterprisePrivate bool
	if *ea || (*eaPub && *eaPriv) || (!*ea && !*eaPub && !*eaPriv && !*ca) {
		enterprisePublic = true
		enterprisePrivate = true
	} else if *eaPub {
		enterprisePublic = true
	} else if *eaPriv {
		enterprisePrivate = true
	}

	if !validateEmail(*user) {
		log.Error("'%s' is not a valid ThousandEyes user.", *user)
		os.Exit(0)
	}

	if !validateToken(*token) {
		log.Error("'%s' is not a valid ThousandEyes user API token. Find your token at https://app.thousandeyes.com/settings/account/?section=profile", *token)
		os.Exit(0)
	}

	agents, _ := fetchAgents(*user, *token, enterprise, cloud, ipv4, ipv6, enterprisePublic, enterprisePrivate)

	if strings.ToLower(*output) == IPList {
		outputIPList(agents, *name)
	} else if strings.ToLower(*output) == SubnetListStrict {
		outputSubnetListStrict(agents, *name)
	} else if strings.ToLower(*output) == SubnetListLoose {
		outputSubnetListLoose(agents, *name)
	} else if strings.ToLower(*output) == IPRangeListStrict {
		outputIPRangeListStrict(agents, *name)
	} else if strings.ToLower(*output) == IPRangeListLoose {
		outputIPRangeListLoose(agents, *name)
	} else if strings.ToLower(*output) == IPBlockListStrict {
		outputIPBlockListStrict(agents, *name)
	} else if strings.ToLower(*output) == IPBlockListLoose {
		outputIPBlockListLoose(agents, *name)
	} else if strings.ToLower(*output) == CSV {
		outputCSV(agents)
	} else if strings.ToLower(*output) == JSON {
		outputJSON(agents)
	} else if strings.ToLower(*output) == XML {
		outputXML(agents)
	} else {
		log.Error("Output type '%s' not supported. Supported output types: %s, %s, %s, %s, %s, %s, %s", *output, IPList, SubnetListStrict, SubnetListLoose, IPRangeListStrict, CSV, JSON, XML)
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

func fetchAgents(user, token string, enterprise, cloud, ipv4, ipv6, enterprisePublic, enterprisePrivate bool) ([]Agent, error) {

	type Agents struct {
		Agents []Agent `json:"agents"`
	}

	var agents Agents

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	}
	var netClient = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
	request, _ := http.NewRequest("GET", ApiUrl, nil)
	request.SetBasicAuth(user, token)
	response, err := netClient.Do(request)
	if err != nil {
		log.Error("TE API request error: %s", err.Error())
		os.Exit(0)
	}

	if response.StatusCode == http.StatusOK {
		// yupepeeee
	} else if response.StatusCode == http.StatusUnauthorized {
		log.Error("Invalid credentials provided. (401)")
		os.Exit(0)
	} else if response.StatusCode == http.StatusForbidden {
		log.Error("Your account does not have permissions to view Agents. (403)")
		os.Exit(0)
	} else if response.StatusCode == http.StatusTooManyRequests {
		log.Error("Your are issuing to many API calls. Try again in a minute. (429)")
		os.Exit(0)
	} else if response.StatusCode == http.StatusInternalServerError {
		log.Error("ThousandEyes API internal server error. Try again later. (500)")
		os.Exit(0)
	} else if response.StatusCode == http.StatusServiceUnavailable {
		log.Error("ThousandEyes API us under maintenance. Try again later. (503)")
		os.Exit(0)
	} else {
		log.Error("ThousandEyes API HTTP error: %s", response.Status)
		os.Exit(0)
	}

	err = json.NewDecoder(response.Body).Decode(&agents)
	if err != nil {
		return []Agent{}, err
	}

	if !enterprise || !cloud {
		for i := len(agents.Agents) - 1; i >= 0; i-- {
			agent := agents.Agents[i]
			// Condition to decide if current element has to be deleted:
			if enterprise && (agent.AgentType == Enterprise || agent.AgentType == EnterpriseCluster) {
				// Keep it
			} else if cloud && agent.AgentType == Cloud {
				// Keep it
			} else {
				agents.Agents = append(agents.Agents[:i], agents.Agents[i+1:]...)
			}
		}
	}

	for i, agent := range agents.Agents {
		// Cloud public & Enterprise private addresses
		if (agent.AgentType == Cloud || (agent.AgentType == Enterprise && enterprisePrivate)) && len(agent.IPAddresses) > 0 {
			for _, ip := range agent.IPAddresses {
				if ipv6 && strings.Contains(ip, ":") {
					agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
				} else if ipv4 && strings.Contains(ip, ".") {
					agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
				}
			}
		}
		// Enterprise public addresses
		if enterprisePublic && len(agent.PublicIPAddresses) > 0 {
			for _, ip := range agent.PublicIPAddresses {
				if ipv6 && strings.Contains(ip, ":") {
					agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
				} else if ipv4 && strings.Contains(ip, ".") {
					agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
				}
			}
			for _, clusterMember := range agent.ClusterMembers {
				for _, ip := range clusterMember.PublicIPAddresses {
					if ipv6 && strings.Contains(ip, ":") {
						agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
					} else if ipv4 && strings.Contains(ip, ".") {
						agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
					}
				}
			}
		}
		// Enterprise Cluster private addresses
		if enterprisePrivate && agent.AgentType == EnterpriseCluster && len(agent.ClusterMembers) > 0 {
			for _, clusterMember := range agent.ClusterMembers {
				for _, ip := range clusterMember.IPAddresses {
					if ipv6 && strings.Contains(ip, ":") {
						agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
					} else if ipv4 && strings.Contains(ip, ".") {
						agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
					}
				}
			}
		}
		// Enterprise Cluster public addresses
		if enterprisePublic && agent.AgentType == EnterpriseCluster && len(agent.ClusterMembers) > 0 {
			for _, clusterMember := range agent.ClusterMembers {
				for _, ip := range clusterMember.PublicIPAddresses {
					if ipv6 && strings.Contains(ip, ":") {
						agents.Agents[i].IPv6Addresses = append(agents.Agents[i].IPv6Addresses, net.ParseIP(ip))
					} else if ipv4 && strings.Contains(ip, ".") {
						agents.Agents[i].IPv4Addresses = append(agents.Agents[i].IPv4Addresses, net.ParseIP(ip))
					}
				}
			}
		}
		agents.Agents[i].IPAddresses = []string{}
		agents.Agents[i].PublicIPAddresses = []string{}
		agents.Agents[i].ClusterMembers = []Agent{}
	}

	if !ipv4 || !ipv6 {
		for i := len(agents.Agents) - 1; i >= 0; i-- {
			// Condition to decide if current element has to be deleted:
			if ipv4 && len(agents.Agents[i].IPv4Addresses) > 0 {
				// Keep it
			} else if ipv6 && len(agents.Agents[i].IPv6Addresses) > 0 {
				// Keep it
			} else {
				agents.Agents = append(agents.Agents[:i], agents.Agents[i+1:]...)
			}
		}
	}

	return agents.Agents, nil
}

func outputIPList(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)

	for _, ip := range ips {
		if name {
			agentsWithIP := getAgentsByIP(agents, ip)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			fmt.Printf("%s %s %s\n", pad(ip.String(), 39), ListCommentChar, agentsStr)
		} else {
			fmt.Printf("%s\n", ip.String())
		}
	}

}

func outputSubnetListStrict(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipNets := ipsToSubnetsStrict(ips)

	for _, ipNet := range ipNets {
		s, t := ipNet.Mask.Size()
		if name {
			agentsWithIP := getAgentsBySubnet(agents, ipNet)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			if s == t {
				fmt.Printf("%s %s %s\n", pad(ipNet.IP.String(), 39), ListCommentChar, agentsStr)
			} else {
				fmt.Printf("%s %s %s\n", pad(ipNet.String(), 39), ListCommentChar, agentsStr)
			}
		} else {
			if s == t {
				fmt.Printf("%s\n", ipNet.IP.String())
			} else {
				fmt.Printf("%s\n", ipNet.String())
			}
		}
	}

}

func outputSubnetListLoose(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipNets := ipsToSubnetsLoose(ips)

	for _, ipNet := range ipNets {
		s, t := ipNet.Mask.Size()
		if name {
			agentsWithIP := getAgentsBySubnet(agents, ipNet)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			if s == t {
				fmt.Printf("%s %s %s\n", pad(ipNet.IP.String(), 39), ListCommentChar, agentsStr)
			} else {
				fmt.Printf("%s %s %s\n", pad(ipNet.String(), 39), ListCommentChar, agentsStr)
			}
		} else {
			if s == t {
				fmt.Printf("%s\n", ipNet.IP.String())
			} else {
				fmt.Printf("%s\n", ipNet.String())
			}
		}
	}

}

type IPRange struct {
	StartIP net.IP
	EndIP   net.IP
}

func (ipRange IPRange) Contains(ip net.IP) bool {
	if ip.To4() != nil && ipRange.StartIP.To4() != nil && ipRange.EndIP.To4() != nil {
		// IPv4
		if binary.BigEndian.Uint32(ip.To4()) >= binary.BigEndian.Uint32(ipRange.StartIP.To4()) && binary.BigEndian.Uint32(ip.To4()) <= binary.BigEndian.Uint32(ipRange.EndIP.To4()) {
			return true
		}
	} else if ip.To4() == nil && ipRange.StartIP.To4() == nil && ipRange.EndIP.To4() == nil {
		// IPv6
		if binary.BigEndian.Uint64(ip[0:8]) > binary.BigEndian.Uint64(ipRange.StartIP[0:8]) && binary.BigEndian.Uint64(ip[0:8]) < binary.BigEndian.Uint64(ipRange.EndIP[0:8]) {
			return true
		} else if bytes.Compare(ip[0:8], ipRange.StartIP[0:8]) == 0 && binary.BigEndian.Uint64(ip[0:8]) < binary.BigEndian.Uint64(ipRange.EndIP[0:8]) &&
			binary.BigEndian.Uint64(ip[8:16]) >= binary.BigEndian.Uint64(ipRange.StartIP[8:16]) {
			return true
		} else if binary.BigEndian.Uint64(ip[0:8]) > binary.BigEndian.Uint64(ipRange.StartIP[0:8]) && bytes.Compare(ip[0:8], ipRange.EndIP[0:8]) == 0 &&
			binary.BigEndian.Uint64(ip[8:16]) <= binary.BigEndian.Uint64(ipRange.EndIP[8:16]) {
			return true
		} else if bytes.Compare(ip[0:8], ipRange.StartIP[0:8]) == 0 && bytes.Compare(ip[0:8], ipRange.EndIP[0:8]) == 0 &&
			binary.BigEndian.Uint64(ip[8:16]) >= binary.BigEndian.Uint64(ipRange.StartIP[8:16]) && binary.BigEndian.Uint64(ip[8:16]) <= binary.BigEndian.Uint64(ipRange.EndIP[8:16]) {
			return true
		}
	}
	return false
}
func (ipRange IPRange) String() string {
	if bytes.Compare(ipRange.StartIP, ipRange.EndIP) != 0 {
		return ipRange.StartIP.String() + " - " + ipRange.EndIP.String()
	} else {
		return ipRange.StartIP.String()
	}
}

func outputIPRangeListStrict(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipRanges := ipsToIPRangesStrict(ips)

	for _, ipRange := range ipRanges {
		if name {
			agentsWithIP := getAgentsByIPRange(agents, ipRange)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			fmt.Printf("%s %s %s\n", pad(ipRange.String(), 59), ListCommentChar, agentsStr)
		} else {
			fmt.Printf("%s\n", ipRange.String())
		}
	}

}

func outputIPRangeListLoose(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipRanges := ipsToIPRangesLoose(ips)

	for _, ipRange := range ipRanges {
		if name {
			agentsWithIP := getAgentsByIPRange(agents, ipRange)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			fmt.Printf("%s %s %s\n", pad(ipRange.String(), 59), ListCommentChar, agentsStr)
		} else {
			fmt.Printf("%s\n", ipRange.String())
		}
	}

}

type IPBlock struct {
	StartIP net.IP
	EndIP   net.IP
}

func (ipBlock IPBlock) Contains(ip net.IP) bool {
	if ip.To4() != nil && ipBlock.StartIP.To4() != nil && ipBlock.EndIP.To4() != nil {
		if binary.BigEndian.Uint32(ip.To4()) >= binary.BigEndian.Uint32(ipBlock.StartIP.To4()) && binary.BigEndian.Uint32(ip.To4()) <= binary.BigEndian.Uint32(ipBlock.EndIP.To4()) {
			return true
		}
	} else if ip.To4() == nil && ipBlock.StartIP.To4() == nil && ipBlock.EndIP.To4() == nil {
		if binary.BigEndian.Uint64(ip[0:8]) > binary.BigEndian.Uint64(ipBlock.StartIP[0:8]) && binary.BigEndian.Uint64(ip[0:8]) < binary.BigEndian.Uint64(ipBlock.EndIP[0:8]) {
			return true
		} else if bytes.Compare(ip[0:8], ipBlock.StartIP[0:8]) == 0 && binary.BigEndian.Uint64(ip[0:8]) < binary.BigEndian.Uint64(ipBlock.EndIP[0:8]) &&
			binary.BigEndian.Uint64(ip[8:16]) >= binary.BigEndian.Uint64(ipBlock.StartIP[8:16]) {
			return true
		} else if binary.BigEndian.Uint64(ip[0:8]) > binary.BigEndian.Uint64(ipBlock.StartIP[0:8]) && bytes.Compare(ip[0:8], ipBlock.EndIP[0:8]) == 0 &&
			binary.BigEndian.Uint64(ip[8:16]) <= binary.BigEndian.Uint64(ipBlock.EndIP[8:16]) {
			return true
		} else if bytes.Compare(ip[0:8], ipBlock.StartIP[0:8]) == 0 && bytes.Compare(ip[0:8], ipBlock.EndIP[0:8]) == 0 &&
			binary.BigEndian.Uint64(ip[8:16]) >= binary.BigEndian.Uint64(ipBlock.StartIP[8:16]) && binary.BigEndian.Uint64(ip[8:16]) <= binary.BigEndian.Uint64(ipBlock.EndIP[8:16]) {
			return true
		}
	}
	return false
}
func (ipBlock IPBlock) String() string {
	start4 := ipBlock.StartIP.To4()
	end4 := ipBlock.EndIP.To4()
	if start4 != nil && end4 != nil {
		// IPv4
		if bytes.Compare(ipBlock.StartIP, ipBlock.EndIP) == 0 {
			return ipBlock.StartIP.String()
		} else if start4[3] != end4[3] {
			return strconv.Itoa(int(start4[0])) + "." + strconv.Itoa(int(start4[1])) + "." + strconv.Itoa(int(start4[2])) + ".[" + strconv.Itoa(int(start4[3])) + "-" + strconv.Itoa(int(end4[3])) + "]"
		} else if start4[2] != end4[2] {
			return strconv.Itoa(int(start4[0])) + "." + strconv.Itoa(int(start4[1])) + ".[" + strconv.Itoa(int(start4[2])) + "-" + strconv.Itoa(int(end4[2])) + "]." + strconv.Itoa(int(start4[3]))
		}
	} else {
		// IPv6
		if bytes.Compare(ipBlock.StartIP, ipBlock.EndIP) == 0 {
			return ipBlock.StartIP.String()
		} else {
			var firstStr, startStr, endStr string
			var firstLen int
			for b := 0; b <= 14; b = b + 2 {
				if bytes.Compare(ipBlock.StartIP[:b+2], ipBlock.EndIP[:b+2]) == 0 {
					firstStr = fmt.Sprintf("%s%x", firstStr, binary.BigEndian.Uint16(ipBlock.StartIP[b:b+2])) + ":"
					firstLen = b + 2
				} else {
					startStr = fmt.Sprintf("%s%x", startStr, binary.BigEndian.Uint16(ipBlock.StartIP[b:b+2])) + ":"
					endStr = fmt.Sprintf("%s%x", endStr, binary.BigEndian.Uint16(ipBlock.EndIP[b:b+2])) + ":"
				}
			}
			startStr = startStr[:len(startStr)-1]
			endStr = endStr[:len(endStr)-1]

			// Shorten IPv6
			if !strings.Contains(firstStr, "::") {
				// Go will shorten IP, lets just generate a complete IP
				fakePrefix := ""
				for i := 0; i*2 < firstLen; i++ {
					fakePrefix = fmt.Sprintf("%s%x:", fakePrefix, i+1)
				}
				fakeStartIPStr := fakePrefix + startStr
				fakeEndIPStr := fakePrefix + endStr
				fakeStartIP := net.ParseIP(fakeStartIPStr)
				fakeEndIP := net.ParseIP(fakeEndIPStr)
				fakeStartIPStr = fakeStartIP.String()
				fakeEndIPStr = fakeEndIP.String()
				startStr = fakeStartIPStr[len(fakePrefix):]
				endStr = fakeEndIPStr[len(fakePrefix):]
			}
			if len(startStr) > 1 && len(endStr) > 1 && startStr[0:1] == ":" && endStr[0:1] == ":" && startStr[0:2] != "::" && endStr[0:2] != "::" {
				return firstStr + ":[" + startStr[1:] + "-" + endStr[1:] + "]"
			} else if startStr[:1] == ":" || endStr[:1] == ":" {
				return firstStr[:len(firstStr)-1] + "[:" + startStr + "-:" + endStr + "]"
			} else {
				return firstStr + "[" + startStr + "-" + endStr + "]"
			}
		}
	}
	return ipBlock.StartIP.String()
}

func outputIPBlockListStrict(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipBlocks := ipsToIPBlocksStrict(ips)

	for _, ipBlock := range ipBlocks {
		if name {
			agentsWithIP := getAgentsByIPBlock(agents, ipBlock)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			fmt.Printf("%s %s %s\n", pad(ipBlock.String(), 46), ListCommentChar, agentsStr)
		} else {
			fmt.Printf("%s\n", pad(ipBlock.String(), 46))
		}
	}

}

func outputIPBlockListLoose(agents []Agent, name bool) {

	ips := sortAgentIPs(agents)
	ipBlocks := ipsToIPBlocksLoose(ips)

	for _, ipBlock := range ipBlocks {
		if name {
			agentsWithIP := getAgentsByIPBlock(agents, ipBlock)
			agentsStr := ""
			for _, agent := range agentsWithIP {
				agentsStr = agentsStr + ListSeparatorChar + " " + agent.AgentName
			}
			if len(agentsStr) > 1 {
				agentsStr = agentsStr[2:]
			}
			fmt.Printf("%s %s %s\n", pad(ipBlock.String(), 46), ListCommentChar, agentsStr)
		} else {
			fmt.Printf("%s\n", pad(ipBlock.String(), 46))
		}
	}

}

func outputCSV(agents []Agent) {

	fmt.Printf("Agent ID%sAgent Name%sAgent Type%sLocation%sCountry%s", CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar)
	fmt.Printf("IPv4 Addresses%sIPv4 Subnets (Strict)%sIPv4 Subnets (Loose)%sIPv4 Ranges (Strict)%sIPv4 Ranges (Loose)%sIPv4 Blocks (Strict)%sIPv4 Blocks (Loose)%s", CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar)
	fmt.Printf("IPv6 Addresses%sIPv6 Subnets (Strict)%sIPv6 Subnets (Loose)%sIPv6 Ranges (Strict)%sIPv6 Ranges (Loose)%sIPv6 Blocks (Strict)%sIPv6 Blocks (Loose)\n", CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar, CSVSeparatorChar)

	agents = addDataToAgents(agents)

	for _, agent := range agents {
		fmt.Printf("%s%s\"%s\"%s%s%s\"%s\"%s%s%s", strconv.Itoa(agent.AgentID), CSVSeparatorChar, agent.AgentName, CSVSeparatorChar, agent.AgentType, CSVSeparatorChar, agent.Location, CSVSeparatorChar, agent.CountryID, CSVSeparatorChar)

		ipStr := ""
		if len(agent.IPv4Addresses) > 0 {
			for _, ip := range agent.IPv4Addresses {
				ipStr = ipStr + ip.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv4SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + "\n"
				} else {
					ipStr = ipStr + ipNet.String() + "\n"
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv4SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + "\n"
				} else {
					ipStr = ipStr + ipNet.String() + "\n"
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4RangesStrict) > 0 {
			for _, ipRange := range agent.IPv4RangesStrict {
				ipStr = ipStr + ipRange.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4RangesLoose) > 0 {
			for _, ipRange := range agent.IPv4RangesLoose {
				ipStr = ipStr + ipRange.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv4BlocksStrict {
				ipStr = ipStr + ipBlock.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv4BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv4BlocksLoose {
				ipStr = ipStr + ipBlock.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6Addresses) > 0 {
			for _, ip := range agent.IPv6Addresses {
				ipStr = ipStr + ip.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6SubnetsStrict) > 0 {
			for _, ipNet := range agent.IPv6SubnetsStrict {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + "\n"
				} else {
					ipStr = ipStr + ipNet.String() + "\n"
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6SubnetsLoose) > 0 {
			for _, ipNet := range agent.IPv6SubnetsLoose {
				s, t := ipNet.Mask.Size()
				if s == t {
					ipStr = ipStr + ipNet.IP.String() + "\n"
				} else {
					ipStr = ipStr + ipNet.String() + "\n"
				}
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6RangesStrict) > 0 {
			for _, ipRange := range agent.IPv6RangesStrict {
				ipStr = ipStr + ipRange.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6RangesLoose) > 0 {
			for _, ipRange := range agent.IPv6RangesLoose {
				ipStr = ipStr + ipRange.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv6BlocksStrict {
				ipStr = ipStr + ipBlock.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"%s", ipStr, CSVSeparatorChar)
		ipStr = ""
		if len(agent.IPv6BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv6BlocksLoose {
				ipStr = ipStr + ipBlock.String() + "\n"
			}
			ipStr = ipStr[0 : len(ipStr)-1]
		}
		fmt.Printf("\"%s\"", ipStr)

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
		IPv4Addresses     []string `json:"ipv4Address,omitempty"`
		IPv6Addresses     []string `json:"ipv6Address,omitempty"`
		IPv4SubnetsStrict []string `json:"ipv4SubnetStrict,omitempty"`
		IPv6SubnetsStrict []string `json:"ipv6SubnetStrict,omitempty"`
		IPv4SubnetsLoose  []string `json:"ipv4SubnetLoose,omitempty"`
		IPv6SubnetsLoose  []string `json:"ipv6SubnetLoose,omitempty"`
		IPv4RangesStrict  []string `json:"ipv4RangeStrict,omitempty"`
		IPv6RangesStrict  []string `json:"ipv6RangeStrict,omitempty"`
		IPv4RangesLoose   []string `json:"ipv4RangeLoose,omitempty"`
		IPv6RangesLoose   []string `json:"ipv6RangeLoose,omitempty"`
		IPv4BlocksStrict  []string `json:"ipv4BlockStrict,omitempty"`
		IPv6BlocksStrict  []string `json:"ipv6BlockStrict,omitempty"`
		IPv4BlocksLoose   []string `json:"ipv4BlockLoose,omitempty"`
		IPv6BlocksLoose   []string `json:"ipv6BlockLoose,omitempty"`
	}

	outputAgents := []OutputAgent{}
	agents = addDataToAgents(agents)

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
		if len(agent.IPv4RangesStrict) > 0 {
			for _, ipRange := range agent.IPv4RangesStrict {
				outputAgent.IPv4RangesStrict = append(outputAgent.IPv4RangesStrict, ipRange.String())
			}
		}
		if len(agent.IPv6RangesStrict) > 0 {
			for _, ipRange := range agent.IPv6RangesStrict {
				outputAgent.IPv6RangesStrict = append(outputAgent.IPv6RangesStrict, ipRange.String())
			}
		}
		if len(agent.IPv4RangesLoose) > 0 {
			for _, ipRange := range agent.IPv4RangesLoose {
				outputAgent.IPv4RangesLoose = append(outputAgent.IPv4RangesLoose, ipRange.String())
			}
		}
		if len(agent.IPv6RangesLoose) > 0 {
			for _, ipRange := range agent.IPv6RangesLoose {
				outputAgent.IPv6RangesLoose = append(outputAgent.IPv6RangesLoose, ipRange.String())
			}
		}
		if len(agent.IPv4BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv4BlocksStrict {
				outputAgent.IPv4BlocksStrict = append(outputAgent.IPv4BlocksStrict, ipBlock.String())
			}
		}
		if len(agent.IPv6BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv6BlocksStrict {
				outputAgent.IPv6BlocksStrict = append(outputAgent.IPv6BlocksStrict, ipBlock.String())
			}
		}
		if len(agent.IPv4BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv4BlocksLoose {
				outputAgent.IPv4BlocksLoose = append(outputAgent.IPv4BlocksLoose, ipBlock.String())
			}
		}
		if len(agent.IPv6BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv6BlocksLoose {
				outputAgent.IPv6BlocksLoose = append(outputAgent.IPv6BlocksLoose, ipBlock.String())
			}
		}
		outputAgents = append(outputAgents, outputAgent)
	}

	j, _ := json.MarshalIndent(outputAgents, "", "  ")

	fmt.Printf("%s", string(j))
}

func outputXML(agents []Agent) {

	type OutputAgent struct {
		XMLName           xml.Name `xml:"agent"`
		AgentID           int      `xml:"agentId"`
		AgentName         string   `xml:"agentName"`
		AgentType         string   `xml:"agentType"`
		Location          string   `xml:"location,omitempty""`
		CountryID         string   `xml:"countryId,omitempty""`
		IPv4Addresses     []string `xml:"ipv4Address,omitempty"`
		IPv6Addresses     []string `xml:"ipv6Address,omitempty"`
		IPv4SubnetsStrict []string `xml:"ipv4SubnetStrict,omitempty"`
		IPv6SubnetsStrict []string `xml:"ipv6SubnetStrict,omitempty"`
		IPv4SubnetsLoose  []string `xml:"ipv4SubnetLoose,omitempty"`
		IPv6SubnetsLoose  []string `xml:"ipv6SubnetLoose,omitempty"`
		IPv4RangesStrict  []string `xml:"ipv4RangeStrict,omitempty"`
		IPv6RangesStrict  []string `xml:"ipv6RangeStrict,omitempty"`
		IPv4RangesLoose   []string `xml:"ipv4RangeLoose,omitempty"`
		IPv6RangesLoose   []string `xml:"ipv6RangeLoose,omitempty"`
		IPv4BlocksStrict  []string `xml:"ipv4BlockStrict,omitempty"`
		IPv6BlocksStrict  []string `xml:"ipv6BlockStrict,omitempty"`
		IPv4BlocksLoose   []string `xml:"ipv4BlockLoose,omitempty"`
		IPv6BlocksLoose   []string `xml:"ipv6BlockLoose,omitempty"`
	}

	outputAgents := []OutputAgent{}
	agents = addDataToAgents(agents)

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
		if len(agent.IPv4RangesStrict) > 0 {
			for _, ipRange := range agent.IPv4RangesStrict {
				outputAgent.IPv4RangesStrict = append(outputAgent.IPv4RangesStrict, ipRange.String())
			}
		}
		if len(agent.IPv6RangesStrict) > 0 {
			for _, ipRange := range agent.IPv6RangesStrict {
				outputAgent.IPv6RangesStrict = append(outputAgent.IPv6RangesStrict, ipRange.String())
			}
		}
		if len(agent.IPv4RangesLoose) > 0 {
			for _, ipRange := range agent.IPv4RangesLoose {
				outputAgent.IPv4RangesLoose = append(outputAgent.IPv4RangesLoose, ipRange.String())
			}
		}
		if len(agent.IPv6RangesLoose) > 0 {
			for _, ipRange := range agent.IPv6RangesLoose {
				outputAgent.IPv6RangesLoose = append(outputAgent.IPv6RangesLoose, ipRange.String())
			}
		}
		if len(agent.IPv4BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv4BlocksStrict {
				outputAgent.IPv4BlocksStrict = append(outputAgent.IPv4BlocksStrict, ipBlock.String())
			}
		}
		if len(agent.IPv6BlocksStrict) > 0 {
			for _, ipBlock := range agent.IPv6BlocksStrict {
				outputAgent.IPv6BlocksStrict = append(outputAgent.IPv6BlocksStrict, ipBlock.String())
			}
		}
		if len(agent.IPv4BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv4BlocksLoose {
				outputAgent.IPv4BlocksLoose = append(outputAgent.IPv4BlocksLoose, ipBlock.String())
			}
		}
		if len(agent.IPv6BlocksLoose) > 0 {
			for _, ipBlock := range agent.IPv6BlocksLoose {
				outputAgent.IPv6BlocksLoose = append(outputAgent.IPv6BlocksLoose, ipBlock.String())
			}
		}
		outputAgents = append(outputAgents, outputAgent)
	}

	x, _ := xml.MarshalIndent(outputAgents, "", "  ")

	fmt.Printf("%s", xml.Header)
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

func addDataToAgents(agents []Agent) []Agent {

	for i, agent := range agents {
		if len(agent.IPv4Addresses) > 0 {
			ips := sortIPs(agent.IPv4Addresses)
			agents[i].IPv4SubnetsStrict = ipsToSubnetsStrict(ips)
			agents[i].IPv4SubnetsLoose = ipsToSubnetsLoose(ips)
			agents[i].IPv4RangesStrict = ipsToIPRangesStrict(ips)
			agents[i].IPv4RangesLoose = ipsToIPRangesLoose(ips)
			agents[i].IPv4BlocksStrict = ipsToIPBlocksStrict(ips)
			agents[i].IPv4BlocksLoose = ipsToIPBlocksLoose(ips)
		}
		if len(agent.IPv6Addresses) > 0 {
			ips := sortIPs(agent.IPv6Addresses)
			agents[i].IPv6SubnetsStrict = ipsToSubnetsStrict(ips)
			agents[i].IPv6SubnetsLoose = ipsToSubnetsLoose(ips)
			agents[i].IPv6RangesStrict = ipsToIPRangesStrict(ips)
			agents[i].IPv6RangesLoose = ipsToIPRangesLoose(ips)
			agents[i].IPv6BlocksStrict = ipsToIPBlocksStrict(ips)
			agents[i].IPv6BlocksLoose = ipsToIPBlocksLoose(ips)
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

// Returns true if IPs are sorted by sortIPs()
func ipsSorted(ips []net.IP) bool {

	for i, ip := range ips {
		if i+1 < len(ips) && binary.BigEndian.Uint64(ip[0:8]) > binary.BigEndian.Uint64(ips[i+1][0:8]) {
			return false
		}
		if i+1 < len(ips) && binary.BigEndian.Uint64(ip[8:16]) > binary.BigEndian.Uint64(ips[i+1][8:16]) {
			return false
		}
	}

	return true

}

// Transform a list of IPs to a list of subnets that exactly match the list of
// IPs
// ips []net.IP MUST be sorted by sortIPs()
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
// ips []net.IP MUST be sorted by sortIPs()
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

// Transform a list of IPs to a strict list of IP ranges, i.e. 10.0.0.3 - 10.0.0.5
// ips []net.IP MUST be sorted by sortIPs()
func ipsToIPRangesStrict(ips []net.IP) []IPRange {

	ipRanges := []IPRange{}

	if len(ips) == 0 {
		return ipRanges
	} else if len(ips) == 1 {
		ipRanges = append(ipRanges, IPRange{ips[0], ips[0]})
		return ipRanges
	}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		ipRange := IPRange{ip, ip}

		if ip.To4() != nil {
			// IPv4
			for n := 1; n < len(ips)-i; n++ {
				if ips[i+n].To4() != nil && binary.BigEndian.Uint32(ips[i+n].To4()) == binary.BigEndian.Uint32(ip.To4())+uint32(n) {
					ipRange.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipRanges = append(ipRanges, ipRange)
		} else {
			// IPv6
			for n := 1; n < len(ips)-i; n++ {
				// First 64 bits have to be equal, last 64 bits must be one after another
				if ips[i+n].To4() == nil && bytes.Compare(ips[i+n][0:8], ip[0:8]) == 0 && binary.BigEndian.Uint64(ips[i+n][8:16]) == binary.BigEndian.Uint64(ip[8:16])+uint64(n) {
					ipRange.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipRanges = append(ipRanges, ipRange)
		}
	}

	return ipRanges

}

// Transform a list of IPs to a loose list of IP ranges, i.e.
// 10.0.0.3, 10.0.0.5 -> 10.0.0.3 - 10.0.0.5
// ips []net.IP MUST be sorted by sortIPs()
func ipsToIPRangesLoose(ips []net.IP) []IPRange {

	ipRanges := []IPRange{}

	if len(ips) == 0 {
		return ipRanges
	} else if len(ips) == 1 {
		ipRanges = append(ipRanges, IPRange{ips[0], ips[0]})
		return ipRanges
	}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		ipRange := IPRange{ip, ip}

		if ip.To4() != nil {
			// IPv4
			for n := 1; n < len(ips)-i; n++ {
				// IPs that are less than 255 apart are joined in a range
				if ips[i+n].To4() != nil && binary.BigEndian.Uint32(ips[i+n].To4())-binary.BigEndian.Uint32(ip.To4()) < uint32(n*255) {
					ipRange.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipRanges = append(ipRanges, ipRange)
		} else {
			// IPv6
			for n := 1; n < len(ips)-i; n++ {
				// Put anything in the same /64 subnet to the same range
				if ips[i+n].To4() == nil && bytes.Compare(ips[i+n][0:8], ip[0:8]) == 0 {
					ipRange.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipRanges = append(ipRanges, ipRange)
		}
	}

	return ipRanges

}

// Transform a list of IPs to a strict list of IP blocks, i.e.
// 10.0.0.3, 10.0.0.4 -> 10.0.0.[3-4]
// 10.0.1.3, 10.0.2.3 -> 10.0.[1-2].3
// ips []net.IP MUST be sorted by sortIPs()
func ipsToIPBlocksStrict(ips []net.IP) []IPBlock {

	ipBlocks := []IPBlock{}

	if len(ips) == 0 {
		return ipBlocks
	} else if len(ips) == 1 {
		ipBlocks = append(ipBlocks, IPBlock{ips[0], ips[0]})
		return ipBlocks
	}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		ipBlock := IPBlock{ip, ip}

		if ip.To4() != nil {
			ip4 := ip.To4()
			// IPv4
			for n := 1; n < len(ips)-i; n++ {
				ipN := ips[i+n].To4()
				if ipN != nil && binary.BigEndian.Uint32(ipN) == binary.BigEndian.Uint32(ip4)+uint32(n) {
					// D part of 2 IPs is continguos
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else if ipN != nil && uint8(ipN[2]) == uint8(ip4[2])+uint8(n) && ip4[3] == ipN[3] {
					// C part of 2 IPs is continguos, D part is equal
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipBlocks = append(ipBlocks, ipBlock)
		} else {
			// IPv6
			for n := 1; n < len(ips)-i; n++ {
				// First 64 bits have to be equal, last 64 bits must be one after another
				if ips[i+n].To4() == nil && bytes.Compare(ips[i+n][0:8], ip[0:8]) == 0 && binary.BigEndian.Uint64(ips[i+n][8:16]) == binary.BigEndian.Uint64(ip[8:16])+uint64(n) {
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipBlocks = append(ipBlocks, ipBlock)
		}
	}

	return ipBlocks

}

// Transform a list of IPs to a loose list of IP blocks, i.e.
// 10.0.0.3, 10.0.0.7 -> 10.0.0.[3-7]
// 10.0.1.3, 10.0.3.3 -> 10.0.[1-3].3
// ips []net.IP MUST be sorted by sortIPs()
func ipsToIPBlocksLoose(ips []net.IP) []IPBlock {

	ipBlocks := []IPBlock{}

	if len(ips) == 0 {
		return ipBlocks
	} else if len(ips) == 1 {
		ipBlocks = append(ipBlocks, IPBlock{ips[0], ips[0]})
		return ipBlocks
	}

	iAlreadyCovered := -1
	for i, ip := range ips {
		if i <= iAlreadyCovered {
			continue
		}

		ipBlock := IPBlock{ip, ip}

		if ip.To4() != nil {
			ip4 := ip.To4()
			// IPv4
			for n := 1; n < len(ips)-i; n++ {
				ipN := ips[i+n].To4()
				if ipN != nil && ip4[3] != ipN[3] && ip4[0] == ipN[0] && ip4[1] == ipN[1] && ip4[2] == ipN[2] {
					// D part of 2 IPs different
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else if ipN != nil && ip4[2] != ipN[2] && ip4[0] == ipN[0] && ip4[1] == ipN[1] && ip4[3] == ipN[3] {
					// C part of 2 IPs different
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipBlocks = append(ipBlocks, ipBlock)
		} else {
			// IPv6
			for n := 1; n < len(ips)-i; n++ {
				// First 64 bits have to be equal, last 64 bits must be one after another
				if ips[i+n].To4() == nil && bytes.Compare(ips[i+n][0:8], ip[0:8]) == 0 {
					ipBlock.EndIP = ips[i+n]
					iAlreadyCovered = i + n
				} else {
					break
				}
			}
			ipBlocks = append(ipBlocks, ipBlock)
		}
	}

	return ipBlocks

}

// Returns all agents that have provided IP address
func getAgentsByIP(agents []Agent, ip net.IP) []Agent {
	returnAgents := []Agent{}

	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 && ip.To4() != nil {
			for _, aip := range agent.IPv4Addresses {
				if bytes.Compare(ip, aip) == 0 {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		} else if len(agent.IPv6Addresses) > 0 && ip.To4() == nil {
			for _, aip := range agent.IPv6Addresses {
				if bytes.Compare(ip, aip) == 0 {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		}
	}

	return returnAgents
}

// Returns all agents that have an IP inside provided subnet
func getAgentsBySubnet(agents []Agent, ipNet net.IPNet) []Agent {
	returnAgents := []Agent{}

	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 && ipNet.IP.To4() != nil {
			for _, aip := range agent.IPv4Addresses {
				if ipNet.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		} else if len(agent.IPv6Addresses) > 0 && ipNet.IP.To4() == nil {
			for _, aip := range agent.IPv6Addresses {
				if ipNet.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		}
	}

	return returnAgents
}

// Returns all agents that have an IP inside provided IPRange
func getAgentsByIPRange(agents []Agent, ipRange IPRange) []Agent {
	returnAgents := []Agent{}

	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 && ipRange.StartIP.To4() != nil {
			for _, aip := range agent.IPv4Addresses {
				if ipRange.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		} else if len(agent.IPv6Addresses) > 0 && ipRange.StartIP.To4() == nil {
			for _, aip := range agent.IPv6Addresses {
				if ipRange.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		}
	}

	return returnAgents
}

// Returns all agents that have an IP inside provided ipBlock block
func getAgentsByIPBlock(agents []Agent, ipBlock IPBlock) []Agent {
	returnAgents := []Agent{}
	for _, agent := range agents {
		if len(agent.IPv4Addresses) > 0 && ipBlock.StartIP.To4() != nil {
			for _, aip := range agent.IPv4Addresses {
				if ipBlock.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
				aip4 := aip.To4()
				sip4 := ipBlock.StartIP.To4()
				eip4 := ipBlock.EndIP.To4()
				if aip4 != nil && sip4 != nil && eip4 != nil &&
					uint8(aip4[2]) >= uint8(sip4[2]) && uint8(aip4[2]) <= uint8(eip4[2]) && aip[3] == sip4[3] && aip[3] == eip4[3] {
					// C part of 2 IPv4s is continguos, D part is equal
					returnAgents = append(returnAgents, agent)
					break
				}

			}
		} else if len(agent.IPv6Addresses) > 0 && ipBlock.StartIP.To4() == nil {
			for _, aip := range agent.IPv6Addresses {
				if ipBlock.Contains(aip) {
					returnAgents = append(returnAgents, agent)
					break
				}
			}
		}
	}

	return returnAgents
}

func pad(str string, totalLen int) string {
	var padLen int
	if len(str) < totalLen {
		padLen = totalLen - len(str)
	}
	for x := 0; x < padLen; x++ {
		str = str + " "
	}
	return str
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
