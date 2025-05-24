// Copyright 2017 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

type myIP struct {
	IP   string
	Port string
	ASN  string
}

func fuzzyMyIP(ip string) string {
	ipr, _ := netip.ParseAddr(ip)
	bl := 24
	if ipr.Is6() {
		bl = 56
	}
	ipp, _ := ipr.Prefix(bl)
	return ipp.String()
}

func asnRIPE(ip string) string {
	// https://stat.ripe.net/data/prefix-overview/data.json?resource=9.9.9.9/24

	ip, _ = strings.CutPrefix(ip, "[")
	ip, _ = strings.CutSuffix(ip, "]")

	baseURL := "https://stat.ripe.net/data/prefix-overview/data.json?resource=%s"
	url := fmt.Sprintf(baseURL, ip)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "asnRIPE: %s for %s\n", err, url)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "asnRIPE: %d for %s\n", resp.StatusCode, url)
		return ""
	}

	type Response struct {
		Data struct {
			Asns []struct {
				Asn    int    `json:"asn"`
				Holder string `json:"holder"`
			} `json:"asns"`
		} `json:"data"`
	}

	response := Response{}
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&response)

	if len(response.Data.Asns) == 0 {
		fmt.Fprintf(os.Stderr, "asnRIPE: 0 asns for %s\n", ip)
		return ""
	}

	asn := response.Data.Asns[0]

	return "ASN" + strconv.Itoa(asn.Asn) + " " + asn.Holder
}

// MyIPHandler yells back the remote IP to the browser
func MyIPHandler(infoAPI string, fuzzy bool) http.Handler {

	retrieveASN := func(mi *myIP) *myIP { return mi }
	switch infoAPI {
	case "ripe":
		retrieveASN = func(mi *myIP) *myIP { mi.ASN = asnRIPE(mi.IP); return mi }
		break
	default:
		break
	}

	fuzzyIP := func(mi *myIP) *myIP { return mi }
	if fuzzy {
		fuzzyIP = func(mi *myIP) *myIP { mi.IP = fuzzyMyIP(mi.IP); return mi }
	}

	tmpl, _ := template.New("myip").Parse(`<!doctype html>
<html>
	<head>
		<title>knut - myip</title>
	</head>
	<body>
		<p>Your IP is: <span id="ip">{{ .IP }}</span>:<span id="port">{{.Port}}</span></p>
		{{ if .ASN -}}
		<p>Your ASN is: <span id="asn">{{ .ASN }}</span></p>
		{{ end }}
	</body>
</html>`)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, port, _ := net.SplitHostPort(r.RemoteAddr)
		myip := &myIP{IP: ip, Port: port}
		myip = retrieveASN(myip)
		myip = fuzzyIP(myip)
		tmpl.Execute(w, myip)
	})
}
