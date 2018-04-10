package main

import (
	"fmt"
	"net"
)

func main() {
	ips, err := net.LookupIP("v5test_central.2.")
	fmt.Println(err)
	fmt.Println(ips)
}
