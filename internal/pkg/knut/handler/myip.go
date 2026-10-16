// Copyright 2017 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

type myIP struct {
	view.Page
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
		slog.Warn("asn lookup", "url", url, "err", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("asn lookup", "url", url, "status", resp.StatusCode)
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
		slog.Warn("asn lookup: no asns", "ip", ip)
		return ""
	}

	asn := response.Data.Asns[0]

	return "ASN" + strconv.Itoa(asn.Asn) + " " + asn.Holder
}

// MyIPHandler yells back the remote IP to the browser
func MyIPHandler(infoAPI string, fuzzy bool) http.Handler {

	retrieveASN := func(mi *myIP) *myIP { return mi }
	if infoAPI == "ripe" {
		retrieveASN = func(mi *myIP) *myIP { mi.ASN = asnRIPE(mi.IP); return mi }
	}

	fuzzyIP := func(mi *myIP) *myIP { return mi }
	if fuzzy {
		fuzzyIP = func(mi *myIP) *myIP { mi.IP = fuzzyMyIP(mi.IP); return mi }
	}

	tmpl := view.Template("myip")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, port, _ := net.SplitHostPort(r.RemoteAddr)
		myip := &myIP{Page: view.PageFor(r, "myip"), IP: ip, Port: port}
		myip = retrieveASN(myip)
		myip = fuzzyIP(myip)
		view.WriteFor(w, r, tmpl, myip)
	})
}
