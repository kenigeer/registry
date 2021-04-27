// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/apigee/registry/server"
	"gopkg.in/yaml.v3"
)

var config server.Config

func main() {
	configFlag := flag.String("c", "config/registry.yaml", "specify a configuration file")
	flag.Parse()
	if path := *configFlag; path != "" {
		fi, err := os.Lstat(path)
		if err != nil {
			log.Fatalf("Failed to read file info: %s", err)
		}

		// Follow symbolic links to a readable config file if applicable.
		if (fi.Mode() & os.ModeSymlink) != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				log.Fatalf("Failed to read symbolic link %q: %s", path, err)
			}
			path = target
		}

		b, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalf("Failed to read file: %s", err)
		}

		if err := yaml.Unmarshal(b, &config); err != nil {
			log.Fatalf("Failed to unmarshal yaml: %s", err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	err := server.RunServer(":"+port, &config)
	if err != nil {
		log.Fatalf("Failed to start server: %s", err.Error())
	}
}
