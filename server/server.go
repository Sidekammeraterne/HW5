package main

import (
	proto "AuctionServer/grpc"
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
)

type AuctionServer struct {
	proto.UnimplementedAuctionServer

	id     int    //todo: maybe not used?
	role   string // the role of the server, leader or backup
	backup proto.AuctionClient
	//other servers??? Yes I believe so

	state   *AuctionState
	lamport int32
}

//we need N server nodes - min 2
//one leader at a time

// holding replicated data
type AuctionState struct {
	//todo: lock?
	duration      int32
	auctionClosed bool

	highestBid     int32
	highestBidder  int32
	registeredBids map[int32]bool //set bidderID to true? todo: should be registeredBidders
}

func (s *AuctionServer) Bid(ctx context.Context, in *proto.Amount) (*proto.Ack, error) {
	s.updateLamportOnReceive(in.Lamport)
	if s.state.auctionClosed {
		log.Printf("Bid by %v of %d caused exception as the auction is closed", in.Id, in.Amount)
		return &proto.Ack{Outcome: "exception"}, nil
	}

	if in.Amount < s.state.highestBid {
		log.Printf("Bid by %v of %d fail as it was not a valid bid", in.Id, in.Amount)
		return &proto.Ack{Outcome: "fail"}, nil
	}

	s.state.registeredBids[in.Id] = true
	s.state.highestBidder = in.Id
	s.state.highestBid = in.Amount
	log.Printf("Bid by %v of %d was successfully added to the Auction", in.Id, in.Amount)

	// Update backups if leader
	if s.role == "leader" {
		Ack, err := s.backup.Bid(ctx, in)
		if err != nil {
			log.Fatalf("Did not work: %v", err)
		}
		log.Printf("Acknowlegdement from backup was recieved with value: %v", Ack)
	}

	s.incrementLamport()
	return &proto.Ack{Outcome: "success"}, nil
}

func (s *AuctionServer) Result(ctx context.Context, in *proto.Empty) (*proto.Outcome, error) {
	s.updateLamportOnReceive(in.Lamport)
	s.incrementLamport()
	return &proto.Outcome{Id: s.state.highestBidder, HighestBid: s.state.highestBid, ActionClosed: s.state.auctionClosed}, nil
}

func main() {
	server := &AuctionServer{} //todo: set server properties, role should be given in the terminal

	if server.role == "leader" {
		auction := AuctionState{
			duration:       100,
			auctionClosed:  false,
			highestBid:     0,
			highestBidder:  0,
			registeredBids: make(map[int32]bool),
		}
		server.state = &auction
	}

	//todo: make client connection to the other server

	server.startServer()
}

func (s *AuctionServer) startServer() {
	grpcServer := grpc.NewServer()
	listner, err := net.Listen("tcp", ":8080") //todo: fix hardcoded port
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	proto.RegisterAuctionServer(grpcServer, s)

	err = grpcServer.Serve(listner)
	if err != nil {
		log.Fatalf("Did not work: %v", err)
	}
}

// utility method that compares the received lamport clock with the local one and updates it if the received one is higher
func (s *AuctionServer) updateLamportOnReceive(remoteLamport int32) int32 {
	if remoteLamport > s.lamport {
		s.lamport = remoteLamport
		s.checkLamport()
	}
	s.incrementLamport()
	s.checkLamport()
	return s.lamport
}

func (s *AuctionServer) checkLamport() {
	if s.lamport >= s.state.duration {
		s.state.auctionClosed = true
	}
}

// utility method that increments the local lamport clock
func (s *AuctionServer) incrementLamport() {
	s.lamport++
	s.checkLamport()
}

//what needs to happen:
//registered bidders, current highest bid and who made the bid
//some kind of time, 100 time units, from starting the program - Done (not tested)

//BACKUP: replicate state from the leader
//if leader fails (figured out using timeout)?
//backup promotes itself to leader
//other backups now replicate the new leader
