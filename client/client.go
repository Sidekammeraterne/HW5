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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "AuctionServer/grpc"
)

//needs to know who the leader server is (or maybe all)
//sending bid(amount) to server

// represents a bidder in the auction, holding ID and the gRPC used to call RPC methods on the server
type Client struct {
	ID      int32
	Server  proto.AuctionClient
	Lamport int32
}

func main() {
	//Flags
	addrFlag := flag.String("addr", "localhost:5050", "the auction address") //defaulting to port 5050
	idFlag := flag.Int("id", 1, "the bidder/client ID")                      //defaults to id
	flag.Parse()

	//Connecting to server
	conn, err := grpc.NewClient(*addrFlag, grpc.WithTransportCredentials(insecure.NewCredentials())) //todo: obs. hardoded port
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	//creates a new instance of the object client
	c := Client{
		ID:     int32(*idFlag),
		Server: proto.NewAuctionClient(conn),
	}

	fmt.Printf("Connected to auction as client %d", c.ID)
	fmt.Println("Commands: bid <amount> | result | quit") //what the user can type into terminal

	//start listening for commands in terminal
	c.listenCommands() //todo: like in ChitChat
}

func (c *Client) incrementLamport() {
	c.Lamport++
}
func (c *Client) updateLamortOnReceive(remote int32) {
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
	//todo: timeout?
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()

	//proto message
	req := &proto.Amount{
		Id:      c.ID,      //bidder ID
		Amount:  amount,    //bid amount
		Lamport: c.Lamport, //lamport
	}
	//send rpc
	response, err := c.Server.Bid(context.Background(), req)
	if err != nil {
		log.Printf("Server not responding: %v", err)
		return err
	}
	//update local lamport from server reply
	c.updateLamortOnReceive(response.GetLamport())
	fmt.Printf("Bid %d from client %d had outcome %s\n", amount, c.ID, response.GetOutcome())
	return nil
}

// get state of auction from server, gets hughest bid or result
func (c *Client) Result() error {
	c.incrementLamport()
	//todo: timeout?
	response, err := c.Server.Result(context.Background(), &proto.Empty{})
	if err != nil {
		return err
	}

	c.updateLamortOnReceive(response.GetLamport())

	status := "open"
	if response.GetActionClosed() {
		status = "closed"
	}
	fmt.Printf("Result of auction -> highestBid=%d, winnerId=%d, auction=%s \n", response.GetHighestBid(), response.GetId(), status)
	return nil
}
