// Code generated by model-compiler. DO NOT EDIT.

// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"github.com/onosproject/aether-models/models/aether-2.0.x/v2/api"
    "github.com/onosproject/config-models/pkg/path"
	"github.com/onosproject/config-models/pkg/xpath/navigator"
	"github.com/onosproject/onos-api/go/onos/config/admin"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-lib-go/pkg/northbound"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"os"
	"strconv"
)

var log = logging.GetLogger("plugin")

type modelPlugin struct {
}

type server struct {
}

// gRPC path lists; derived from native path maps
var roPaths []*admin.ReadOnlyPath
var rwPaths []*admin.ReadWritePath

func (p *modelPlugin) Register(gs *grpc.Server) {
	log.Info("Registering model plugin service")
	server := &server{}
	admin.RegisterModelPluginServiceServer(gs, server)
}

func main() {
	ready := make(chan bool)

	if len(os.Args) < 2 {
		log.Fatal("gRPC port argument is required")
		os.Exit(1)
	}

	i, err := strconv.ParseInt(os.Args[1], 10, 16)
	if err != nil {
		log.Fatal("specified gRPC port is invalid", err)
		os.Exit(1)
	}
	port := int16(i)

	entries, err := api.UnzipSchema()
	if err != nil {
		log.Fatalf("Unable to extract model schema: %+v", err)
	}
	roPaths, rwPaths = path.ExtractPaths(entries)

	// Start gRPC server
	log.Info("Starting model plugin")
	p := modelPlugin{}
	if err := p.startNorthboundServer(port); err != nil {
		log.Fatal("Unable to start model plugin service", err)
	}

	// Serve
	<-ready
}

func (p *modelPlugin) startNorthboundServer(port int16) error {
	cfg := northbound.NewServerConfig("", "", "", port, true)
	s := northbound.NewServer(cfg)

	s.AddService(p)

	doneCh := make(chan error)
	go func() {
		err := s.Serve(func(started string) {
			log.Info("Started NBI on ", started)
			close(doneCh)
		})
		if err != nil {
			doneCh <- err
		}
	}()
	return <-doneCh
}

func (s server) GetModelInfo(ctx context.Context, request *admin.ModelInfoRequest) (*admin.ModelInfoResponse, error) {
	log.Infof("Received model info request: %+v", request)
	return &admin.ModelInfoResponse{
		ModelInfo: &admin.ModelInfo{
			Name:               "aether",
			Version:            "2.0.x",
			ModelData:          api.ModelData(),
			SupportedEncodings: api.Encodings(),
			GetStateMode:       0,
			ReadOnlyPath:       roPaths,
			ReadWritePath:      rwPaths,
		},
	}, nil
}

func (s server) ValidateConfig(ctx context.Context, request *admin.ValidateConfigRequest) (*admin.ValidateConfigResponse, error) {
	log.Infof("Received validate config request: %s", request.String())
	gostruct, err := s.unmarshallConfigValues(request.Json)
	if err != nil {
		return nil, errors.Status(err).Err()
	}

	if err := s.validate(gostruct); err != nil {
		return nil, errors.Status(err).Err()
	}

	if err := s.validateMust(*gostruct); err != nil {
		return nil, errors.Status(err).Err()
	}
	return &admin.ValidateConfigResponse{Valid: true}, nil
}

func (s server) GetPathValues(ctx context.Context, request *admin.PathValuesRequest) (*admin.PathValuesResponse, error) {
	log.Infof("Received path values request: %+v", request)
	pathValues, err := path.GetPathValues(request.PathPrefix, request.Json)
	if err != nil {
		return nil, errors.Status(errors.NewInvalid("Unable to get path values: %+v", err)).Err()
	}
	return &admin.PathValuesResponse{PathValues: pathValues}, nil
}

func (s server) unmarshallConfigValues(jsonTree []byte) (*ygot.ValidatedGoStruct, error) {
	device := &api.Device{}
	vgs := ygot.ValidatedGoStruct(device)
	if err := api.Unmarshal([]byte(jsonTree), device); err != nil {
		return nil, errors.NewInvalid("Unable to unmarshal JSON: %+v", err)
	}
	return &vgs, nil
}

func (s server) validate(ygotModel *ygot.ValidatedGoStruct, opts ...ygot.ValidationOption) error {
	deviceDeref := *ygotModel
	device, ok := deviceDeref.(*api.Device)
	if !ok {
		return errors.NewInvalid("Unable to convert model aether-2.0.x")
	}
	return device.Validate()
}

func (s server) validateMust(device ygot.ValidatedGoStruct) error {
	log.Infof("Received validateMust request for device: %v", device)
	schema, err := api.Schema()
	if err != nil {
		return errors.NewInvalid("Unable to get schema: %+v", err)
	}

	nn := navigator.NewYangNodeNavigator(schema.RootSchema(), device)
	ynn, ok := nn.(*navigator.YangNodeNavigator)
	if !ok {
		return errors.NewInvalid("Cannot cast NodeNavigator to YangNodeNavigator")
	}
	return ynn.WalkAndValidateMust()
}

