package raft
import "time"
import "net"
import "net/rpc"
import "net/http"
import "sync"
const (
	electionTimeOut = 20 * time.Millisecond
	heartBeatTimeOut = 10 * time.Millisecond

	follower = iota
	candidate
	leader
)

// This state is maintained by all nodes in raft cluster
type nodeState struct{
// persistent values
	currentTerm uint64
	votedFor string
	log Log
// volatile	values
	commitIdx uint64
	lastApplied uint64
}

// This state is maintained by leader of raft cluster
type leaderState struct{
		NextIndex map[net.Addr]uint64
		MatchIndex map[net.Addr]uint64
}

// This is a single node in raft cluster
type Node struct{
	myAddr net.Addr
	members []net.Addr
	peerConnections map[net.Addr]*rpc.Client
	connectionMapMu sync.Mutex
	state uint32
	done  bool
	//fan-in channel for all rpc messages
	msgChan chan RpcMsg
	listener net.Listener

	nodeState nodeState
	leaderState  leaderState
}

// Returns a new node in raft cluster
func NewNode(members []net.Addr, myAddr net.Addr) (*Node, error){
	node :=  &Node {
							myAddr : myAddr,
							members :RemoveAddr(members, myAddr),
							peerConnections : make(map[net.Addr]*rpc.Client),
							state : follower, // every node begins as a follower
							msgChan : make(chan RpcMsg),
							}
	// start rpc server
	rpc.Register(node)
	rpc.HandleHTTP()
	l,e := net.Listen("tcp", myAddr.String())
	if e != nil{
		return node, e
	}
	node.listener = l
	go http.Serve(l, nil)
	return node, nil
}

func(node *Node) getClient( addr net.Addr) *rpc.Client{
	node.connectionMapMu.Lock()
	defer node.connectionMapMu.Unlock()
	var client *rpc.Client
	if cl,ok := node.peerConnections[addr]; !ok{
		if client,err := rpc.DialHTTP("tcp", addr.String()); err != nil{
        node.peerConnections[addr] = client
      }
	}else{
		client = cl
	}
	return client
}

func(node *Node) follower(){
	for node.state == follower{

	}
}

func(node *Node) candidate(){
	for node.state == candidate{

	}
}

func(node *Node) leader(){
	for node.state == leader{

	}

}

func(node *Node) start(){
		for node.done == false {
			switch node.state{
				case follower:
					node.follower()
				case candidate:
					node.candidate()
				case leader:
					node.leader()
			}
		}
}


func(node * Node) Kill(){
	node.done = true
	node.listener.Close()
}


