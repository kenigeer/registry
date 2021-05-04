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

package names

import (
	"fmt"
	"regexp"
)

// Api represents a resource name for an API.
type Api struct {
	ProjectID string
	ApiID     string
}

// Validate returns an error if the resource name is invalid.
// For backward compatibility, names should only be validated at creation time.
func (a Api) Validate() error {
	r := ApiRegexp()
	if name := a.String(); !r.MatchString(name) {
		return fmt.Errorf("invalid API name %q: must match %q", name, r)
	}

	return validateID(a.ApiID)
}

// Project returns the name of this resource's parent project.
func (a Api) Project() Project {
	return Project{
		ProjectID: a.ProjectID,
	}
}

// Version returns an API version with the provided ID and this resource as its parent.
func (a Api) Version(id string) Version {
	return Version{
		ProjectID: a.ProjectID,
		ApiID:     a.ApiID,
		VersionID: normalize(id),
	}
}

// Artifact returns an artifact with the provided ID and this resource as its parent.
func (a Api) Artifact(id string) Artifact {
	return Artifact{
		name: apiArtifact{
			ProjectID:  a.ProjectID,
			ApiID:      a.ApiID,
			ArtifactID: normalize(id),
		},
	}
}

func (a Api) String() string {
	return normalize(fmt.Sprintf("projects/%s/apis/%s", a.ProjectID, a.ApiID))
}

// ApisRegexp returns a regular expression that matches collection of apis.
func ApisRegexp() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf("^projects/%s/apis$", identifier))
}

// ApiRegexp returns a regular expression that matches a api resource name.
func ApiRegexp() *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf("^projects/%s/apis/%s$", identifier, identifier))
}

// ParseApi parses the name of an Api.
func ParseApi(name string) (Api, error) {
	r := ApiRegexp()
	if !r.MatchString(name) {
		return Api{}, fmt.Errorf("invalid API name %q: must match %q", name, r)
	}

	m := r.FindStringSubmatch(normalize(name))
	return Api{
		ProjectID: m[1],
		ApiID:     m[2],
	}, nil
}
