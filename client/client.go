package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "AuctionServer/grpc"
)

//needs to know who the leader server is (or maybe all)
//sending bid(amount) to server

// represents a bidder in the auction, holding ID and the gRPC used to call RPC methods on the server
type Client struct {
	ID           int32
	Server       proto.AuctionClient
	Backup       string
	Lamport      int32
	AmountOfBids int32
}

type Config struct {
	ID      int32
	Servers []string
}

func parseConfig() Config {
	id := flag.Int("id", 1, "bidder ID")
	servers := flag.String("servers", ":8081", "comma separated list of servers")
	flag.Parse()

	var serverList []string
	if *servers != "" {
		serverList = strings.Split(*servers, ",")
	}

	return Config{
		ID:      int32(*id),
		Servers: serverList,
	}
}
func main() {
	cfg := parseConfig()

	//try first server
	addr := cfg.Servers[0]

	//Connecting to server
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	//creates a new instance of the object client
	c := Client{
		ID:           cfg.ID,
		Server:       proto.NewAuctionClient(conn),
		Backup:       cfg.Servers[1],
		AmountOfBids: 0,
	}

	fmt.Printf("Connected to auction as client %d on server %s", c.ID, addr)
	fmt.Println("Commands: bid <amount> | result | quit") //what the user can type into terminal

	//start listening for commands in terminal
	c.listenCommands()
}

func (c *Client) incrementLamport() {
	c.Lamport++
}
func (c *Client) updateLamportOnReceive(remote int32) {
	if remote > c.Lamport {
		c.Lamport = remote
	}
	c.incrementLamport()
}

// handles user input from terminal
func (c *Client) listenCommands() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		//error handling
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		//trimming spaces off
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		//split into commmand + amount argument, command is always first
		parts := strings.Split(line, " ")
		cmd := parts[0]

		switch cmd {
		case "bid":
			if len(parts) != 2 {
				fmt.Println("needs amount, try again")
				continue
			}
			//convert bid to int
			amountInt, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("amount must be an integer")
				continue
			}
			//send bid to server
			if err := c.Bid(int32(amountInt)); err != nil {
				fmt.Println("error in bid", err)
			}
			//todo: other failure cases?
		case "result":
			//get auction result
			if err := c.Result(); err != nil {
				fmt.Println("error in result", err)
			}
		case "quit":
			fmt.Println("Quitting")
			return
		default:
			fmt.Print("unknown command, valid commands: bid <amount> | result | quit")
		}
	}
}

// Sends a bid(amount) RPC to the server
func (c *Client) Bid(amount int32) error {
	c.incrementLamport()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.AmountOfBids++

	//proto message
	req := &proto.Amount{
		Id:           c.ID,           //bidder ID
		Amount:       amount,         //bid amount
		Lamport:      c.Lamport,      //lamport
		AmountOfBids: c.AmountOfBids, //amount of bids
	}
	//send rpc
	response, err := c.Server.Bid(ctx, req)
	if err != nil {
		log.Printf("Server not responding")
		c.LeaderNotResponding()
		log.Printf("Trying backup")
		response, err := c.Server.Bid(ctx, req)
		response = response
		if err != nil {
			log.Printf("No servers not responding: %v", err)
			c.AmountOfBids--
			return err
		}

	}
	//update local lamport from server reply
	c.updateLamportOnReceive(response.GetLamport())
	fmt.Printf("Bid %d from client %d had outcome %s\n", amount, c.ID, response.GetOutcome())
	return nil
}

// get state of auction from server, get highest bid or result
func (c *Client) Result() error {
	c.incrementLamport()
	//todo: timeout?
	response, err := c.Server.Result(context.Background(), &proto.Empty{})
	if err != nil {
		return err
	}

	c.updateLamportOnReceive(response.GetLamport())

	status := "open"
	if response.GetActionClosed() {
		status = "closed"
	}
	fmt.Printf("Result of auction -> highestBid=%d, winnerId=%d, auction=%s \n", response.GetHighestBid(), response.GetId(), status)
	return nil
}

func (c *Client) LeaderNotResponding() {
	//Connecting to backup server
	conn, err := grpc.NewClient(c.Backup, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	//Update server
	c.Server = proto.NewAuctionClient(conn)

	log.Printf("Server updated to %v", c.Backup)
}
