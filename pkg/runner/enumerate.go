package runner

import (
	"net"
	"os"
	"time"

	"github.com/projectdiscovery/naabu/pkg/log"
	"github.com/projectdiscovery/naabu/pkg/scan"
)

// EnumerateSingleHost performs port enumeration against a single host
func (r *Runner) EnumerateSingleHost(host string, ports map[int]struct{}, output string, add bool) {
	var hostIP string

	// If the host is a Domain, then perform resolution and discover all IP
	// addresses for a given host. Else use that host for port scanning
	var initialHosts []string

	if net.ParseIP(host) == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Warningf("Could not get IP for host: %s\n", host)
			return
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				initialHosts = append(initialHosts, ip.String())
			}
		}

		if len(initialHosts) == 0 {
			log.Warningf("No IP addresses found for host: %s\n", host)
			return
		}
		log.Debugf("Found %v addresses for %s\n", initialHosts, host)
	} else {
		initialHosts = append(initialHosts, host)
		log.Debugf("Using IP address %s for enumeration\n", host)
	}

	// If the user has specified ping probes, perform ping on addresses
	if r.options.Ping {
		// Scan the hosts found for ping probes
		pingResults, err := scan.PingHosts(initialHosts)
		if err != nil {
			log.Warningf("Could not perform ping scan on %s: %s\n", host, err)
			return
		}
		for _, result := range pingResults.Hosts {
			if result.Type == scan.HostActive {
				log.Debugf("Ping probe succeed for %s: latency=%s\n", result.Host, result.Latency)
			} else {
				log.Debugf("Ping probe failed for %s: error=%s\n", result.Host, result.Error)
			}
		}

		// Get the fastest host in the list of hosts
		fastestHost, err := pingResults.GetFastestHost()
		if err != nil {
			log.Warningf("No active host found for %s: %s\n", host, err)
			return
		}
		log.Infof("Fastest host found for target: %s (%s)\n", fastestHost.Host, fastestHost.Latency)
		hostIP = fastestHost.Host
	} else {
		hostIP = initialHosts[0]
		log.Infof("Using host %s for enumeration\n", host)
	}

	log.Infof("Starting scan on host %s (%s)\n", host, hostIP)

	scanner, err := scan.NewScanner(net.ParseIP(hostIP), time.Duration(r.options.Timeout)*time.Millisecond, r.options.Retries, r.options.Rate)
	if err != nil {
		log.Warningf("Could not start scan on host %s (%s): %s\n", host, hostIP, err)
		return
	}
	results, err := scanner.Scan(ports)
	if err != nil {
		log.Warningf("Could not scan on host %s (%s): %s\n", host, hostIP, err)
		return
	}

	if len(results) == 0 {
		log.Infof("No ports found on %s (%s). Host seems down\n", host, hostIP)
		return
	}

	// Validate the host if the user has asked for second step validation
	if r.options.Verify {
		results = scanner.ConnectVerify(host, results)
	}
	log.Infof("Found %d ports on host %s (%s)\n", len(results), host, hostIP)

	for port := range results {
		log.Silentf("%s:%d\n", host, port)
	}

	// In case the user has given an output file, write all the found
	// ports to the output file.
	if output != "" {
		// If the output format is json, append .json
		// else append .txt
		if r.options.OutputDirectory != "" {
			if r.options.JSON {
				output = output + ".json"
			} else {
				output = output + ".txt"
			}
		}

		var file *os.File
		var err error
		if add {
			file, err = os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		} else {
			file, err = os.Create(output)
		}
		if err != nil {
			log.Errorf("Could not create file %s for %s: %s\n", output, host, err)
			return
		}

		// Write the output to the file depending upon user requirement
		if r.options.JSON {
			err = WriteJSONOutput(host, results, file)
		} else {
			err = WriteHostOutput(host, results, file)
		}
		if err != nil {
			log.Errorf("Could not write results to file %s for %s: %s\n", output, host, err)
		}
		file.Close()
		return
	}
	return
}
