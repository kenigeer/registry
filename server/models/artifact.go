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

package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/apigee/registry/rpc"
	"github.com/apigee/registry/server/names"
	"github.com/golang/protobuf/ptypes"
)

// Artifact is the storage-side representation of an artifact.
type Artifact struct {
	Key         string    `datastore:"-" gorm:"primaryKey"`
	ProjectID   string    // Project associated with artifact (required).
	ApiID       string    // Api associated with artifact (if appropriate).
	VersionID   string    // Version associated with artifact (if appropriate).
	SpecID      string    // Spec associated with artifact (if appropriate).
	RevisionID  string    // Spec revision id (if appropriate).
	ArtifactID  string    // Artifact identifier (required).
	CreateTime  time.Time // Creation time.
	UpdateTime  time.Time // Time of last change.
	MimeType    string    // MIME type of artifact
	SizeInBytes int32     // Size of the spec.
	Hash        string    // A hash of the spec.
}

// NewArtifactFromParentAndArtifactID returns an initialized artifact for a specified parent and artifactID.
func NewArtifactFromParentAndArtifactID(parent string, artifactID string) (*Artifact, error) {
	// Return an error if the artifactID is invalid.
	if err := names.ValidateID(artifactID); err != nil {
		return nil, err
	}
	// Match regular expressions to identify the parent of this artifact.
	var m []string
	// Is the parent a project?
	m = names.ProjectRegexp().FindStringSubmatch(parent)
	if m != nil {
		return &Artifact{
			ProjectID:  m[1],
			ArtifactID: artifactID,
		}, nil
	}
	// Is the parent a api?
	m = names.ApiRegexp().FindStringSubmatch(parent)
	if m != nil {
		return &Artifact{
			ProjectID:  m[1],
			ApiID:      m[2],
			ArtifactID: artifactID,
		}, nil
	}
	// Is the parent a version?
	m = names.VersionRegexp().FindStringSubmatch(parent)
	if m != nil {
		return &Artifact{
			ProjectID:  m[1],
			ApiID:      m[2],
			VersionID:  m[3],
			ArtifactID: artifactID,
		}, nil
	}
	// Is the parent a spec?
	m = names.SpecRegexp().FindStringSubmatch(parent)
	if m != nil {
		return &Artifact{
			ProjectID:  m[1],
			ApiID:      m[2],
			VersionID:  m[3],
			SpecID:     m[4],
			ArtifactID: artifactID,
		}, nil
	}
	// Return an error for an unrecognized parent.
	return nil, fmt.Errorf("invalid parent '%s'", parent)
}

// NewArtifactFromResourceName parses resource names and returns an initialized artifact.
func NewArtifactFromResourceName(name string) (*Artifact, error) {
	// split name into parts
	parts := strings.Split(name, "/")
	if len(parts) < 2 || parts[len(parts)-2] != "artifacts" {
		return nil, fmt.Errorf("invalid artifact name '%s'", name)
	}
	// build artifact from parent and artifactID
	parent := strings.Join(parts[0:len(parts)-2], "/")
	artifactID := parts[len(parts)-1]
	return NewArtifactFromParentAndArtifactID(parent, artifactID)
}

// Name returns the resource name of the artifact.
func (artifact *Artifact) Name() string {
	switch {
	case artifact.SpecID != "":
		return fmt.Sprintf("projects/%s/apis/%s/versions/%s/specs/%s/artifacts/%s",
			artifact.ProjectID, artifact.ApiID, artifact.VersionID, artifact.SpecID, artifact.ArtifactID)
	case artifact.VersionID != "":
		return fmt.Sprintf("projects/%s/apis/%s/versions/%s/artifacts/%s",
			artifact.ProjectID, artifact.ApiID, artifact.VersionID, artifact.ArtifactID)
	case artifact.ApiID != "":
		return fmt.Sprintf("projects/%s/apis/%s/artifacts/%s",
			artifact.ProjectID, artifact.ApiID, artifact.ArtifactID)
	case artifact.ProjectID != "":
		return fmt.Sprintf("projects/%s/artifacts/%s",
			artifact.ProjectID, artifact.ArtifactID)
	default:
		return "UNKNOWN"
	}
}

// FullMessage returns the full view of the artifact resource as an RPC message.
func (artifact *Artifact) FullMessage(blob *Blob) (message *rpc.Artifact, err error) {
	message, err = artifact.BasicMessage()
	if err != nil {
		return nil, err
	}

	message.Contents = blob.Contents
	return message, nil
}

// BasicMessage returns the basic view of the artifact resource as an RPC message.
func (artifact *Artifact) BasicMessage() (message *rpc.Artifact, err error) {
	message = &rpc.Artifact{
		Name:      artifact.Name(),
		MimeType:  artifact.MimeType,
		SizeBytes: artifact.SizeInBytes,
		Hash:      artifact.Hash,
	}

	message.CreateTime, err = ptypes.TimestampProto(artifact.CreateTime)
	if err != nil {
		return nil, err
	}

	message.UpdateTime, err = ptypes.TimestampProto(artifact.UpdateTime)
	if err != nil {
		return nil, err
	}

	return message, nil
}

// Update modifies an artifact using the contents of a message.
func (artifact *Artifact) Update(message *rpc.Artifact) {
	artifact.UpdateTime = time.Now()
	artifact.MimeType = message.MimeType
	artifact.SizeInBytes = int32(len(message.Contents))
	artifact.Hash = hashForBytes(message.Contents)
}
