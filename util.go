package raft
import "net"

func RemoveAddr(addressList []net.Addr, removeAddress net.Addr) []net.Addr{
	removedList := make([]net.Addr, 0,  len(addressList))
	for _,address := range addressList {
		if address.String() != removeAddress.String() {
				removedList = append(removedList, address)
		}
	}
	return removedList
}

