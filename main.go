package main

import (
	"gameserver/backend"
	"gameserver/proto"
	"gameserver/server"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"net"
)

func main() {

	lis, err := net.Listen("tcp", "localhost:15001")
	if err != nil {
		logrus.Fatal(err)
	}

	s := grpc.NewServer()
	ng := backend.NewGame()
	srv := server.NewGameServer(ng, "password")
	proto.RegisterGameBackendServer(s, srv)

	srv.LocalChecks()

	if err := s.Serve(lis); err != nil {
		logrus.Fatal("failed to serve: %v", err)
	}

}
