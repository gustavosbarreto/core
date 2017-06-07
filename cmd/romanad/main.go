// Copyright (c) 2016 Pani Networks
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

// Command for running the IPAM service.
package main

import (
	"fmt"
	"github.com/romana/core/common"
	"github.com/romana/core/server"
)

// Main entry point for the IPAM microservice
func main() {
	cs := common.NewCliState()
	ipam := &ipam.IPAMSvc{}
	svcInfo, err := cs.StartService(ipam)
	if err != nil {
		panic(err)
	}
	if svcInfo != nil {
		for {
			msg := <-svcInfo.Channel
			fmt.Println(msg)
		}
	}
}