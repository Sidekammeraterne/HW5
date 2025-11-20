package main

import (
	proto "AuctionServer/grpc"
	"context"
	"flag"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type AuctionServer struct {
	proto.UnimplementedAuctionServer

	role    string // the role of the server, leader or backup
	backup  proto.AuctionClient
	state   *AuctionState
	lamport int32
}

type Config struct {
	Role            string //leader or backup
	Port            string //:xxxx
	OtherServerPort string //localhost:xxxx
}

func parseConfig() Config {
	role := flag.String("role", "leader", "server role")
	port := flag.String("port", ":8080", "listen address")
	other := flag.String("otherServer", "", "other address")
	flag.Parse()
	return Config{*role, *port, *other}
}

// holding replicated data
type AuctionState struct {
	duration      int32
	auctionClosed bool

	highestBid    int32
	highestBidder int32
}

func (s *AuctionServer) Bid(ctx context.Context, in *proto.Amount) (*proto.Ack, error) {
	//check if call comes from leader or client
	if s.role != "leader" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			val := md.Get("source")
			if val[0] == "client" {
				s.role = "leader"
				log.Print("Changed state to leader")
			}
		}
	}

	s.updateLamportOnReceive(in.Lamport)
	if s.state.auctionClosed {
		log.Printf("Bid by %v of %d caused exception as the auction is closed", in.Id, in.Amount)
		return &proto.Ack{Outcome: "exception"}, nil
	}

	if in.AmountOfBids == 1 {
		log.Printf("Bidder %v is registered and can now bid (time=%d)", in.Id, s.lamport)
	}

	if in.Amount <= s.state.highestBid {
		log.Printf("Bid by %v of %d fail as it was not a valid bid", in.Id, in.Amount)
		return &proto.Ack{Outcome: "fail"}, nil
	}

	s.state.highestBidder = in.Id
	s.state.highestBid = in.Amount
	log.Printf("Bid by %v of %d was successfully added to the Auction (time=%d)", in.Id, in.Amount, s.lamport)

	// Update backup if leader
	if s.role == "leader" && s.backup != nil {
		s.incrementLamport()
		//meta data
		md := metadata.Pairs("source", "leader")
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		//construct update
		req := &proto.Amount{
			Id:           in.Id,           //bidder ID
			Amount:       in.Amount,       //bid amount
			Lamport:      s.lamport,       //lamport
			AmountOfBids: in.AmountOfBids, //amount of bids
		}

		Ack, err := s.backup.Bid(ctx, req)
		if err != nil {
			log.Printf("Backup is not responding")
			s.backup = nil
		}
		log.Printf("Ack from backup was recieved: %v", Ack)
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
	cfg := parseConfig()

	server := &AuctionServer{
		role: cfg.Role,
	}

	auction := AuctionState{
		duration:      50,
		auctionClosed: false,
		highestBid:    0,
		highestBidder: 0,
	}
	server.state = &auction

	if server.role == "leader" {
		// Make client connection to the other server
		clientAddress := cfg.OtherServerPort
		conn, err := grpc.NewClient(clientAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		log.Printf("Connected to %v as ", clientAddress)
		if err != nil {
			log.Fatalf("Not working")
		}
		client := proto.NewAuctionClient(conn)
		server.backup = client
		log.Printf("Set backup to %v", server.backup)
	}

	server.startServer(cfg.Port)
}

func (s *AuctionServer) startServer(port string) {
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	proto.RegisterAuctionServer(grpcServer, s)

	log.Printf("Auction server listening on port %v (time=%d)", port, s.lamport)
	err = grpcServer.Serve(listener)
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
