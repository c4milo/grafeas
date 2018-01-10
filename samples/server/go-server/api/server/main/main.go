// Copyright 2017 The Grafeas Authors. All rights reserved.
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
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/cockroachdb/cmux"
	"github.com/grafeas/grafeas/samples/server/go-server/api/server/storage"
	"github.com/grafeas/grafeas/samples/server/go-server/api/server/v1alpha1"
	pb "github.com/grafeas/grafeas/v1alpha1/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	opspb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

func main() {
	flag.Parse()
	grafeasEndpoint := fmt.Sprintf("localhost:%d", *port)

	Run(grafeasEndpoint, nil, "", "")
}

// Run initializes grpc and grpc gateway api services on the same address
func Run(Addr string, tlsConfig *tls.Config, CertFile, KeyFile string) {
	l, err := net.Listen("tcp", Addr)
	if err != nil {
		log.Fatalf("could not listen to address" + Addr)
	}
	log.Println("starting grpc server")

	var (
		apiHandler  http.Handler
		apiListener net.Listener
		srv         *http.Server
		ctx         = context.Background()
		httpMux     = http.NewServeMux()
		tcpMux      = cmux.New(l)
	)

	if tlsConfig != nil {
		cert, err := tls.LoadX509KeyPair(CertFile, KeyFile)
		if err != nil {
			log.Fatal("Failed to load certificate files")
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.NextProtos = []string{"h2"}

		apiListener = tls.NewListener(tcpMux.Match(cmux.Any()), tlsConfig)
		go func() { handleShutdown(tcpMux.Serve()) }()

		grpcServer := newGrpcServer(tlsConfig)
		gwmux := newGrpcGatewayServer(ctx, apiListener.Addr().String(), tlsConfig)

		httpMux.Handle("/", gwmux)
		apiHandler = grpcHandlerFunc(grpcServer, httpMux)

		log.Println("grpc server is configured with client certificate authentication")
	} else {
		grpcL := tcpMux.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
		apiListener = tcpMux.Match(cmux.Any())
		go func() { handleShutdown(tcpMux.Serve()) }()

		grpcServer := newGrpcServer(nil)
		go func() { handleShutdown(grpcServer.Serve(grpcL)) }()

		gwmux := newGrpcGatewayServer(ctx, apiListener.Addr().String(), nil)

		httpMux.Handle("/", gwmux)
		apiHandler = httpMux

		log.Println("grpc server is configured without client certificate authentication")
	}

	srv = &http.Server{
		Handler:   apiHandler,
		TLSConfig: tlsConfig,
	}

	// blocking call
	handleShutdown(srv.Serve(apiListener))
	log.Println("Grpc API stopped")
}

// handleShutdown handles the server shut down error.
func handleShutdown(err error) {
	if err != nil {
		if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
			log.Fatal(err)
		}
	}
}

func newGrpcServer(tlsConfig *tls.Config) *grpc.Server {
	grpcOpts := []grpc.ServerOption{}

	if tlsConfig != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	g := v1alpha1.Grafeas{S: storage.NewMemStore()}
	pb.RegisterGrafeasServer(grpcServer, &g)
	pb.RegisterGrafeasProjectsServer(grpcServer, &g)
	opspb.RegisterOperationsServer(grpcServer, &g)

	return grpcServer
}

func newGrpcGatewayServer(ctx context.Context, listenerAddr string, tlsConfig *tls.Config) http.Handler {
	var (
		gwTLSConfig *tls.Config
		gwOpts      []grpc.DialOption
	)

	if tlsConfig != nil {
		gwTLSConfig = tlsConfig.Clone()
		gwTLSConfig.InsecureSkipVerify = true
		gwOpts = append(gwOpts, grpc.WithTransportCredentials(credentials.NewTLS(gwTLSConfig)))
	} else {
		gwOpts = append(gwOpts, grpc.WithInsecure())
	}

	// changes json serializer to include empty fields with default values
	jsonOpt := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{EmitDefaults: true})
	gwmux := runtime.NewServeMux(jsonOpt)

	conn, err := grpc.DialContext(ctx, listenerAddr, gwOpts...)
	if err != nil {
		log.Fatal("could not initialize grpc gateway connection")
	}

	err = pb.RegisterGrafeasHandler(ctx, gwmux, conn)
	if err != nil {
		log.Fatal("could not initialize ancestry grpc gateway")
	}

	err = pb.RegisterGrafeasProjectsHandler(ctx, gwmux, conn)
	if err != nil {
		log.Fatal("could not initialize notification grpc gateway")
	}

	return http.Handler(gwmux)
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Copied from cockroachdb.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}
