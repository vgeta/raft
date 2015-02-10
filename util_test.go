package raft

import "testing"
import "net"

func TestRemoveAddr(t *testing.T) {
	addr1, err := net.ResolveTCPAddr("tcp", "localhost:1234")
	if err != nil {
		t.Fatalf("unable to resolve tcp address")
	}
	addressList := []net.Addr{addr1}
	newList := RemoveAddr(addressList, addr1)
	if len(newList) != 0 {
		t.Fatalf("expected len of 0 but got %d", len(newList))
	}

	addr2, _ := net.ResolveTCPAddr("tcp", "localhost:1235")
	addressList = []net.Addr{addr1, addr2}
	newList = RemoveAddr(addressList, addr1)
	if len(newList) != 1 {
		t.Fatalf("expected len of 1 but got %d", len(newList))
	}
}
