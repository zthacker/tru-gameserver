package server

import (
	"context"
	"errors"
	"gameserver/backend"
	pb "gameserver/proto"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/zthacker/dungeongenerator"
	"google.golang.org/grpc/metadata"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type client struct {
	streamServer   pb.GameBackend_StreamServer
	lastMessage    time.Time
	done           chan error
	player         uuid.UUID
	id             uuid.UUID
	playerInfo     *backend.Player
	currentDungeon *dungeongenerator.Room
}

type GameServer struct {
	pb.UnimplementedGameBackendServer
	game     *backend.Game
	clients  map[uuid.UUID]*client
	mu       sync.RWMutex
	password string
}

func NewGameServer(game *backend.Game, password string) *GameServer {
	server := &GameServer{
		game:     game,
		clients:  make(map[uuid.UUID]*client),
		password: password,
	}
	return server
}

func (s *GameServer) Stream(srv pb.GameBackend_StreamServer) error {
	ctx := srv.Context()
	currentClient, err := s.getClientFromContext(ctx)
	if err != nil {
		return nil
	}

	if currentClient.streamServer != nil {
		return errors.New("stream already active")
	}
	currentClient.streamServer = srv

	logrus.Infof("starting new stream for %s with id: %s", currentClient.playerInfo.Name, currentClient.player)

	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				logrus.Errorf("receive error %v", err)
				currentClient.done <- errors.New("failed to receive a request")
				return
			}
			logrus.Infof("got a message %v", req)
			currentClient.lastMessage = time.Now()

			switch req.GetAction().(type) {
			case *pb.Request_Move:
				//TODO clean this up
				move := req.GetMove()
				logrus.Infof("%s is wanting to move: %s and is current in room: %d ", currentClient.playerInfo.Name, move.Direction, currentClient.currentDungeon.GetRoom())
				switch move.Direction.String() {
				case "UP":
					if currentClient.currentDungeon.GetNorthDoor() != nil {
						currentClient.currentDungeon = currentClient.currentDungeon.GetNorthDoor()
						logrus.Infof("%s moved to Room: %d", currentClient.playerInfo.Name, currentClient.currentDungeon.GetRoom())

					} else {
						logrus.Infof("%s did not %s because there is not a door there!", currentClient.playerInfo.Name, move.Direction.String())
					}
				case "RIGHT":
					if currentClient.currentDungeon.GetEastDoor() != nil {
						currentClient.currentDungeon = currentClient.currentDungeon.GetEastDoor()
						logrus.Infof("%s moved to Room: %d", currentClient.playerInfo.Name, currentClient.currentDungeon.GetRoom())
					} else {
						logrus.Infof("%s did not %s because there is not a door there!", currentClient.playerInfo.Name, move.Direction.String())
					}
				case "DOWN":
					if currentClient.currentDungeon.GetSouthDoor() != nil {
						currentClient.currentDungeon = currentClient.currentDungeon.GetSouthDoor()
						logrus.Infof("%s moved to Room: %d", currentClient.playerInfo.Name, currentClient.currentDungeon.GetRoom())
					} else {
						logrus.Infof("%s did not %s because there is not a door there!", currentClient.playerInfo.Name, move.Direction.String())
					}
				case "LEFT":
					if currentClient.currentDungeon.GetWestDoor() != nil {
						currentClient.currentDungeon = currentClient.currentDungeon.GetWestDoor()
						logrus.Infof("%s moved to Room: %d", currentClient.playerInfo.Name, currentClient.currentDungeon.GetRoom())
					} else {
						logrus.Infof("%s did not %s because there is not a door there!", currentClient.playerInfo.Name, move.Direction.String())
					}
				}
			}
		}
	}()

	var doneError error
	select {
	case <-ctx.Done():
		doneError = ctx.Err()
	case doneError = <-currentClient.done:
	}

	logrus.Errorf("stream done with error %v", doneError)

	logrus.Infof("%s - removing client", currentClient.id)
	s.removeClient(currentClient.id)

	return nil
}

func (s *GameServer) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	//TODO do I need a game server client limit? Do a check to see if it's "full" or not

	if req.Password != "password" {
		return nil, errors.New("password did not match")
	}

	playerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, err
	}

	//check if player exists
	s.game.Mu.RLock()
	if s.game.GetEntity(playerID) != nil {
		return nil, errors.New("player already exists")
	}
	s.game.Mu.RUnlock()

	// create player
	//TODO remove the rand int as it's only there for testing leader board stuff
	lvl := rand.Intn(100)
	player := &backend.Player{Name: req.Name, IdentifierBase: backend.IdentifierBase{UUID: playerID}, Level: lvl}
	s.game.Mu.Lock()
	s.game.AddEntity(player)
	s.game.Mu.Unlock()

	//make a slice of all the entities to send back
	s.game.Mu.RLock()
	entities := make([]*pb.Entity, 0)
	for _, entity := range s.game.Entities {
		protoEntity := pb.GetProtoEntity(entity)
		if protoEntity != nil {
			entities = append(entities, protoEntity)
		}
	}
	s.game.Mu.RUnlock()

	// broadcast to other clients about the new player
	//resp := pb.Response{
	//	Action: &pb.Response_AddEntity{
	//		AddEntity: &pb.AddEntity{
	//			Entity: pb.GetProtoEntity(player),
	//		},
	//	},
	//}
	//s.broadcast(&resp)

	// Add the new client to the game server
	dung := dungeongenerator.NewDungeon(0)
	dung.CreateDungeon(dung, 20)
	s.mu.Lock()
	token := uuid.New()
	s.clients[token] = &client{
		id:             token,
		player:         playerID,
		playerInfo:     player,
		done:           make(chan error),
		lastMessage:    time.Now(),
		currentDungeon: dung,
	}
	s.mu.Unlock()

	return &pb.ConnectResponse{Token: token.String(), Entities: entities}, nil
}

func (s *GameServer) getClientFromContext(ctx context.Context) (*client, error) {
	headers, ok := metadata.FromIncomingContext(ctx)
	tokenRaw := headers["authorization"]
	if len(tokenRaw) == 0 {
		return nil, errors.New("no token provided")
	}
	token, err := uuid.Parse(tokenRaw[0])
	if err != nil {
		return nil, errors.New("cannot parse token")
	}
	s.mu.RLock()
	currentClient, ok := s.clients[token]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.New("token not recognized")
	}

	return currentClient, nil
}

func (s *GameServer) removeClient(id uuid.UUID) {
	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()
}

func (s *GameServer) broadcast(resp *pb.Response) {
	s.mu.Lock()
	for id, currentClient := range s.clients {
		if currentClient.streamServer == nil {
			continue
		}
		if err := currentClient.streamServer.Send(resp); err != nil {
			logrus.Errorf("%s broadcast error %v", id, err)
		}
		logrus.Infof("%s broadcasted %v", id, resp)
	}
	s.mu.Unlock()
}

// LocalChecks is used to dev use to run some timers and test some things
func (s *GameServer) LocalChecks() {

	go func() {
		for {
			//logrus.Infof("Amount of Players on Server: %d", len(s.clients))
			s.CheckLeaderboard()
			time.Sleep(10 * time.Second)
		}
	}()

}

// CheckLeaderboard is a dev function to just test out some leader board ideas that would be sent to clients
func (s *GameServer) CheckLeaderboard() {
	//logrus.Info("checking leader board")
	leaderMap := make(map[int]string)
	s.mu.Lock()
	for _, v := range s.clients {
		leaderMap[v.playerInfo.Level] = v.playerInfo.Name
	}
	s.mu.Unlock()

	keys := make([]int, 0, len(leaderMap))

	for k := range leaderMap {
		keys = append(keys, k)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	//for _, k := range keys {
	//	logrus.Info(k, leaderMap[k])
	//}

}
