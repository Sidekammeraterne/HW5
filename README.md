# HW5
This repository is for Distributed System Homework 5

#leader
go run ./server -role=leader -port=":8080" -otherServer="localhost:8081"

#backup
go run ./server -role=backup -port=":8081" -otherServer="localhost:8080"

# client 1
go run ./client -id=1 -servers="localhost:8080,localhost:8081"

# client 2
go run ./client -id=2 -servers="localhost:8080,localhost:8081"