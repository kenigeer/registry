package controller

import (
	"context"
	"fmt"
	"testing"
	"log"

	"github.com/apigee/registry/connection"
	"github.com/apigee/registry/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/apigee/registry/cmd/registry/core"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var sortStrings = cmpopts.SortSlices(func(a, b string) bool { return a < b })
const gzipOpenAPIv3 = "application/x.openapi+gzip;version=3.0.0"

func deleteProject(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	projectID string) {
	t.Helper()
	req := &rpc.DeleteProjectRequest{
		Name: "projects/" + projectID,
	}
	err := client.DeleteProject(ctx, req)
	if err!= nil && status.Code(err) != codes.NotFound {
		t.Fatalf("Failed DeleteProject(%v): %s", req, err.Error())
	}
}

func createProject(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	projectID string) {
	t.Helper()
	req := &rpc.CreateProjectRequest{
		ProjectId: projectID,
		Project: &rpc.Project{
			DisplayName: "Demo",
			Description: "A demo catalog",
		},
	}
	project, err := client.CreateProject(ctx, req)
	if err != nil {
		t.Fatalf("Failed CreateProject(%v): %s", req, err.Error())
	}
	if project.GetName() != "projects/" + "controller-demo" {
		t.Fatalf("Invalid project name %s", project.GetName())
	}
}

func createApi(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	parent, apiID string) {
	t.Helper()
	req := &rpc.CreateApiRequest{
			Parent: parent,
			ApiId:  apiID,
			Api: &rpc.Api{
				DisplayName:  apiID,
				Description:  fmt.Sprintf("Sample Test API: %s", apiID),
				Availability: "GENERAL",
			},
		}
		_, err := client.CreateApi(ctx, req)
		if err != nil {
			t.Fatalf("Failed CreateApi(%v): %s", req, err.Error())
		}
}

func createVersion(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	parent, versionID string) {
	t.Helper()
	req := &rpc.CreateApiVersionRequest{
			Parent:       parent,
			ApiVersionId: versionID,
			ApiVersion:   &rpc.ApiVersion{},
		}
		_, err := client.CreateApiVersion(ctx, req)
		if err != nil {
			t.Fatalf("Failed CreateApiVersion(%v): %s", req, err.Error())
		}
}

func createSpec(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	parent string,
	specId string,
	mimeType string,
	) {
	t.Helper()
	// Create a spec entry with empty data
	req := &rpc.CreateApiSpecRequest{
		Parent:    parent,
		ApiSpecId: specId,
		ApiSpec: &rpc.ApiSpec{
			MimeType: mimeType,
		},
	}
	_, err := client.CreateApiSpec(ctx, req)
	if err != nil {
		t.Fatalf("Failed CreateApiSpec(%v): %s", req, err.Error())
	}
}

func updateSpec(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	specName string) {
	t.Helper()
	req := &rpc.UpdateApiSpecRequest{
		ApiSpec: &rpc.ApiSpec{
			Name:     specName,
			MimeType: gzipOpenAPIv3,
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"mime_type"}},
	}
	_, err := client.UpdateApiSpec(ctx, req)
	if err != nil {
		t.Fatalf("Failed UpdateApiSpec(%v): %s", req, err.Error())
	}
}

func createUpdateArtifact(
	ctx context.Context,
	client connection.Client,
	t *testing.T,
	artifactName string) {
	t.Helper()
	// Creates an artifact entry with empty data
	artifact := &rpc.Artifact{
		Name:     artifactName,
	}
	err := core.SetArtifact(ctx, client, artifact)
	if err != nil {
		t.Fatalf("Failed SetArtifact(%v): %s", artifact, err.Error())
	}
}

// Tests for artifacts and resources and specs as dependencies

func TestSingleSpec(t *testing.T) {
	// Setup: Single spec in the project
	// Expect: One single command to compute artifact
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()
	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_1.yaml")
	if err != nil {
		t.Error(err.Error())
	}

	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{"compute lint projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml --linter gnostic"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}

func TestMultipleSpecs(t *testing.T) {
	// Setup: 3 specs in project
	// Expect: Create artifact command from scratch for each spec

	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()
	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_1.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	"compute lint projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml --linter gnostic",
	"compute lint projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml --linter gnostic",
	"compute lint projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml --linter gnostic"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}

func TestPartiallyExistingArtifacts(t *testing.T) {
	// Setup: 3 specs in project
	// Artifact already exists for one of the specs
	// Expect: Create artifact command for the remaining two specs

	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic")
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	
	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_1.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	"compute lint projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml --linter gnostic",
	"compute lint projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml --linter gnostic"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}

func TestOutdatedArtifacts(t *testing.T) {
	// Setup: 3 specs in project, 2 artifacts already exist, one of them is outdated
	// Expect: Create artifact command for the non-existing and the outdated artifacts.

	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic")
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic")
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	// Update spec 1.0.1 to make the artifact outdated
	updateSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml")

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_1.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	"compute lint projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml --linter gnostic",
	"compute lint projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml --linter gnostic"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}

// Tests for aggregated artifacts at api level and specs as resources
func TestApiLevelArtifactsCreate(t *testing.T) {
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "test-api-1")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)

	// Test API 2
	createApi(ctx, registryClient, t, "projects/controller-demo", "test-api-2")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)


	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_2.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	"compute vocabulary projects/controller-demo/apis/test-api-1",
	"compute vocabulary projects/controller-demo/apis/test-api-2"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}

}

func TestApiLevelArtifactsOutdated(t *testing.T) {
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "test-api-1")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/test-api-1/artifacts/vocabulary")

	// Test API 2
	createApi(ctx, registryClient, t, "projects/controller-demo", "test-api-2")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/artifacts/vocabulary")
	// Update underlying spec to make artifact outdated
	updateSpec(ctx, registryClient, t, "projects/controller-demo/apis/test-api-2/versions/1.0.1/specs/openapi.yaml")

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_2.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	"compute vocabulary projects/controller-demo/apis/test-api-2"}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}

}

// Tests for derived artifacts with artifacts as dependencies
func TestDerivedArtifactsCreate(t *testing.T) {
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/complexity")
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/complexity")
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity")

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_3.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/complexity"),
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/complexity"),
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity"),
	}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}

}

func TestDerivedArtifactsMissing(t *testing.T) {
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic")
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/complexity")
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity")

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_3.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/complexity"),
	}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}

func TestDerivedArtifactsOutdated(t *testing.T) {
	ctx := context.Background()
	registryClient, err := connection.NewClient(ctx)
	if err != nil {
		t.Logf("Failed to create client: %+v", err)
		t.FailNow()
	}
	defer registryClient.Close()

	// Setup
	deleteProject(ctx, registryClient, t, "controller-demo")
	createProject(ctx, registryClient, t, "controller-demo")
	createApi(ctx, registryClient, t, "projects/controller-demo", "petstore")
	
	// Version 1.0.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/complexity")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/score")
	// Version 1.0.1
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.0.1")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/complexity")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.1/specs/openapi.yaml/artifacts/score")
	// Version 1.1.0
	createVersion(ctx, registryClient, t, "projects/controller-demo/apis/petstore", "1.1.0")
	createSpec(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0", "openapi.yaml", gzipOpenAPIv3)
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/score")
	


	// Make some artifacts outdated from the above setup
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic")
	createUpdateArtifact(ctx, registryClient, t, "projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity")

	// Test the manifest
	manifest, err := ReadManifest(
		"../test/manifest_3.yaml")
	if err != nil {
		t.Error(err.Error())
	}
	
	actions, err := ProcessManifest(manifest)
	if err != nil {
		log.Printf(err.Error())
	}
	expectedActions := []string{
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.0.0/specs/openapi.yaml/artifacts/complexity"),
	fmt.Sprintf(
		"compute score %s %s",
		"projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/lint-gnostic",
		"projects/controller-demo/apis/petstore/versions/1.1.0/specs/openapi.yaml/artifacts/complexity"),
	}
	if diff := cmp.Diff(expectedActions, actions, sortStrings); diff != "" {
		t.Errorf("ProcessManifest(%+v) returned unexpected diff (-want +got):\n%s", manifest, diff)
	}
}



