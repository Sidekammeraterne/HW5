package server

import (
	proto "HW5/grpc/proto"
	"time"
)

// the role of the server
type Role int //todo: maybe not int?
//we need N server nodes - min 2
//one leader at a time

// holding replicated data
type AuctionState struct {
	//todo: lock?
	auctionStart time.Time //todo: what is Time
	auctionEnd   time.Time

	highestBid     int32
	highestBidder  int32
	registeredBids map[int32]bool //set bidderID to true?
}

//what needs to happen:
//registered bidders, current highest bid and who made the bid
//some kind of time, 100 time units, from starting the program

//BACKUP: replicate state from the leader
//if leader fails (figured out using timeout)?
//backup promotes itself to leader
//other backups now replicate the new leader
