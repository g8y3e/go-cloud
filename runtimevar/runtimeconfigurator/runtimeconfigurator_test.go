// Copyright 2018 The Go Cloud Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtimeconfigurator

import (
	"context"
	"errors"
	"testing"

	"gocloud.dev/internal/testing/setup"
	"gocloud.dev/runtimevar"
	"gocloud.dev/runtimevar/driver"
	"gocloud.dev/runtimevar/drivertest"
	pb "google.golang.org/genproto/googleapis/cloud/runtimeconfig/v1beta1"
	"google.golang.org/grpc/status"
)

// This constant records the project used for the last --record.
// If you want to use --record mode,
// 1. Update this constant to your GCP project name (not number!).
// 2. Ensure that the "Runtime Configuration API" is enabled for your project.
// TODO(issue #300): Use Terraform to get this.
const projectID = "google.com:rvangent-testing-prod"

const (
	// config is the runtimeconfig high-level config that variables sit under.
	config = "go_cloud_runtimeconfigurator_test"
)

func resourceName(name string) ResourceName {
	return ResourceName{
		ProjectID: projectID,
		Config:    config,
		Variable:  name,
	}
}

type harness struct {
	client pb.RuntimeConfigManagerClient
	closer func()
}

func newHarness(t *testing.T) (drivertest.Harness, error) {
	ctx := context.Background()
	conn, done := setup.NewGCPgRPCConn(ctx, t, endPoint, "runtimevar")
	client := pb.NewRuntimeConfigManagerClient(conn)
	rn := resourceName("")
	// Ignore errors if the config already exists.
	_, _ = client.CreateConfig(ctx, &pb.CreateConfigRequest{
		Parent: "projects/" + rn.ProjectID,
		Config: &pb.RuntimeConfig{
			Name:        rn.configPath(),
			Description: t.Name(),
		},
	})
	return &harness{
		client: client,
		closer: func() {
			_, _ = client.DeleteConfig(ctx, &pb.DeleteConfigRequest{Name: rn.configPath()})
			done()
		},
	}, nil
}

func (h *harness) MakeWatcher(ctx context.Context, name string, decoder *runtimevar.Decoder) (driver.Watcher, error) {
	return newWatcher(h.client, resourceName(name), decoder, nil)
}

func (h *harness) CreateVariable(ctx context.Context, name string, val []byte) error {
	rn := resourceName(name)
	_, err := h.client.CreateVariable(ctx, &pb.CreateVariableRequest{
		Parent: rn.configPath(),
		Variable: &pb.Variable{
			Name:     rn.String(),
			Contents: &pb.Variable_Value{Value: val},
		},
	})
	return err
}

func (h *harness) UpdateVariable(ctx context.Context, name string, val []byte) error {
	rn := resourceName(name)
	_, err := h.client.UpdateVariable(ctx, &pb.UpdateVariableRequest{
		Name: rn.String(),
		Variable: &pb.Variable{
			Contents: &pb.Variable_Value{Value: val},
		},
	})
	return err
}

func (h *harness) DeleteVariable(ctx context.Context, name string) error {
	rn := resourceName(name)
	_, err := h.client.DeleteVariable(ctx, &pb.DeleteVariableRequest{Name: rn.String()})
	return err
}

func (h *harness) Close() {
	h.closer()
}

func (h *harness) Mutable() bool { return true }

func TestConformance(t *testing.T) {
	drivertest.RunConformanceTests(t, newHarness, []drivertest.AsTest{verifyAs{}})
}

type verifyAs struct{}

func (verifyAs) Name() string {
	return "verify As"
}

func (verifyAs) SnapshotCheck(s *runtimevar.Snapshot) error {
	var v *pb.Variable
	if !s.As(&v) {
		return errors.New("Snapshot.As failed")
	}
	return nil
}

func (verifyAs) ErrorCheck(err error) error {
	var s *status.Status
	if !runtimevar.ErrorAs(err, &s) {
		return errors.New("runtimevar.ErrorAs failed")
	}
	return nil
}
