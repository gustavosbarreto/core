// Copyright (c) 2016-2017 Pani Networks
// All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Command for running the Kubernetes Listener.
package main

import (
	"flag"
	"fmt"

	"os"
	"strings"

	"github.com/romana/core/common"
	"github.com/romana/core/common/client"
	"github.com/romana/core/listener"
	log "github.com/romana/rlog"
)

func main() {
	endpointsStr := flag.String("etcd-endpoints", client.DefaultEtcdEndpoints, "Comma-separated list of etcd endpoints.")
	host := flag.String("host", "localhost", "Host to listen on.")
	port := flag.Int("port", 9602, "Port to listen on.")
	prefix := flag.String("etcd-prefix", client.DefaultEtcdPrefix, "Prefix to use for etcd data.")
	flag.Parse()

	fmt.Println(common.BuildInfo())

	if endpointsStr == nil {
		log.Errorf("No etcd endpoints specified")
		os.Exit(1)
	}
	endpoints := strings.Split(*endpointsStr, ",")
	listener := &listener.KubeListener{Addr: fmt.Sprintf("%s:%d", *host, *port)}

	pr := *prefix
	if !strings.HasPrefix(pr, "/") {
		pr = "/" + pr
	}
	config := common.Config{EtcdEndpoints: endpoints,
		EtcdPrefix: pr,
	}
	svcInfo, err := common.InitializeService(listener, config)
	if err != nil {
		log.Error(err)
		os.Exit(2)
	}
	if svcInfo != nil {
		for {
			msg := <-svcInfo.Channel
			log.Info(msg)
		}
	}
}
