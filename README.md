# te-iplist
_ThousandEyes Agent IP List_

CLI utility that queries ThousandEyes API for the Agents available for your account and outputs Agent IPs in different forms (IP list, subnet list, IP range list, IP block list) and formats (plain text, CSV, JSON, XML).

## Download

* [Linux](https://github.com/thousandeyes/te-iplist/raw/master/bin/linux-32/te-iplist)
* [macOS](https://github.com/thousandeyes/te-iplist/raw/master/bin/macos/te-iplist)
* [Windows](https://github.com/thousandeyes/te-iplist/raw/master/bin/win/te-iplist.exe)

### Installation

#### Linux / macOS

Make the binary executable:

```
chmod +x te-iplist
```

(Optional) Move it to a folder that is in your $PATH, so you can invoke the command from any folder:

```
sudo cp te-iplist /usr/local/bin/
```

## Usage

You need to be in possession of a valid ThousandEyes account to use this utility.

```
te-iplist -u <user> -t <user-api-token>
```

### Account Groups

#### -a
Users assigned to multiple Account Groups can list the Agents available in a specific Account Group with the ``-a <accountGroupId>`` argument. You can list available Account Group IDs with:

```
te-iplist -u <user> -t <user-api-token> -account-groups
```

If ``-a`` is not provided, user's default Account Group is used.

### Output formats

#### -o ip
List of Agent IP addresses.

#### -o subnet-strict
List of IP networks that strictly cover Agent IP addresses.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
```

are expressed as

```
10.0.0.1
10.0.0.2/31
```

#### -o subnet-loose
List of IP networks that loosely cover Agent IP addresses. While generally more effective than ``-o subnet-strict``, it may cover IP addresses *not* used by ThousandEyes Agents.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
```

are expressed as

```
10.0.0.0/30
```

#### -o range-strict
List of IP ranges that strictly cover Agent IP addresses.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
10.0.0.5
```

are expressed as

```
10.0.0.1 - 10.0.0.3
10.0.0.5
```

#### -o range-loose
List of IP networks that loosely cover Agent IP addresses. While generally more effective than ``-o range-strict``, it may cover IP addresses *not* used by ThousandEyes Agents.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
10.0.0.5
```

are expressed as

```
10.0.0.1 - 10.0.0.5
```

#### -o block-strict
List of IP blocks that strictly cover Agent IP addresses.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
10.0.0.10
10.0.1.20
10.0.2.20
```

are expressed as

```
10.0.0.[1-3]
10.0.0.10
10.0.[1-2].20
```

#### -o block-loose
List of IP blocks that loosely cover Agent IP addresses. While generally more effective than ``-o block-strict``, it may cover IP addresses *not* used by ThousandEyes Agents.
Example:
Agent IP addresses

```
10.0.0.1
10.0.0.2
10.0.0.3
10.0.0.10
10.0.1.20
10.0.2.20
```

are expressed as

```
10.0.0.[1-10]
10.0.[1-2].20
```

#### -o csv
.csv or Comma Separated Values output containing the Agent information and their IP addresses in all above mentioned formats.
Example:

```
Agent ID,Agent Name,Agent Type,Location,Country,IPv4 Addresses,IPv4 Subnets (Strict),IPv4 Subnets (Loose),IPv4 Ranges (Strict),IPv4 Ranges (Loose),IPv4 Blocks (Strict),IPv4 Blocks (Loose),IPv6 Addresses,IPv6 Subnets (Strict),IPv6 Subnets (Loose),IPv6 Ranges (Strict),IPv6 Ranges (Loose),IPv6 Blocks (Strict),IPv6 Blocks (Loose)
24695,"Nagoya, Japan",Cloud,"Aichi, Japan",JP,"1.2.3.37","1.2.3.38","1.2.3.39","1.2.3.37","1.2.3.38/31","1.2.3.36/30","1.2.3.37 - 1.2.3.39","1.2.3.37 - 1.2.3.39","1.2.3.[37-39]","1.2.3.[37-39]","","","","","","",""
```

#### -o json
JSON output containing the Agent information and their IP addresses in all above mentioned formats.
Example:

```
[
{
  "agentId": 24695,
  "agentName": "Nagoya, Japan",
  "agentType": "Cloud",
  "location": "Aichi, Japan",
  "countryId": "JP",
  "ipv4Address": [
    "1.2.3.37",
    "1.2.3.38",
    "1.2.3.39"
  ],
  "ipv4SubnetStrict": [
    "1.2.3.37",
    "1.2.3.38/31"
  ],
  "ipv4SubnetLoose": [
    "1.2.3.36/30"
  ],
  "ipv4RangeStrict": [
    "1.2.3.37 - 1.2.3.39"
  ],
  "ipv4RangeLoose": [
    "1.2.3.37 - 1.2.3.39"
  ],
  "ipv4BlockStrict": [
    "1.2.3.[37-39]"
  ],
  "ipv4BlockLoose": [
    "1.2.3.[37-39]"
  ]
}
]
```

#### -o xml
XML output containing the Agent information and their IP addresses in all above mentioned formats.
Example:

```
<agent>
  <agentId>24695</agentId>
  <agentName>Nagoya, Japan</agentName>
  <agentType>Cloud</agentType>
  <location>Aichi, Japan</location>
  <countryId>JP</countryId>
  <ipv4Address>1.2.3.37</ipv4Address>
  <ipv4Address>1.2.3.38</ipv4Address>
  <ipv4Address>1.2.3.39</ipv4Address>
  <ipv4SubnetStrict>1.2.3.37</ipv4SubnetStrict>
  <ipv4SubnetStrict>1.2.3.38/31</ipv4SubnetStrict>
  <ipv4SubnetLoose>1.2.3.36/30</ipv4SubnetLoose>
  <ipv4RangeStrict>1.2.3.37 - 1.2.3.39</ipv4RangeStrict>
  <ipv4RangeLoose>1.2.3.37 - 1.2.3.39</ipv4RangeLoose>
  <ipv4BlockStrict>1.2.3.[37-39]</ipv4BlockStrict>
  <ipv4BlockLoose>1.2.3.[37-39]</ipv4BlockLoose>
</agent>
```
#### -n
Add Agent name as a comment to ``-o ip``, ``-o subnet-strict``, ``-o subnet-loose``, ``-o range-strict``, ``-o range-loose``, ``-o block-strict`` and ``-o block-loose`` output types.
Example:

```
1.2.3.38/31          # Nagoya, Japan
2.3.4.22             # Brussels, Belgium
```

### Filters

#### -4
Display only Agents with IPv4 addresses

#### -6
Display only Agents with IPv6 addresses

#### -c
Display only Cloud Agents

#### -e
Display only Enterprise Agents

#### -e-public
Display only Enterprise Agents public IP addresses

#### -e-private
Display only Enterprise Agents private IP addresses
