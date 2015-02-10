package raft

import "time"
import "net"
import "net/rpc"
import "net/http"
import "sync"
import "fmt"

const (
	electionTimeOut  = 20 * time.Millisecond
	heartBeatTimeOut = 10 * time.Millisecond

	follower = iota
	candidate
	leader
)

const (
	append_entries_req = iota
	voting_req
)

// This is generic msg passed across nodes in raft embedding the actual msg
type rpcMsg struct {
	msgType uint32
	msg     interface{}
	respCh  chan interface{}
}

// This state is maintained by all nodes in raft cluster
type nodeState struct {
	// persistent values
	currentTerm uint64
	votedFor    string
	log         Log
	// volatile	values
	commitIdx   uint64
	lastApplied uint64
}

// This state is maintained by leader of raft cluster
type leaderState struct {
	NextIndex  map[net.Addr]uint64
	MatchIndex map[net.Addr]uint64
}

// This is a single node in raft cluster
type Node struct {
	myAddr          net.Addr
	members         []net.Addr
	peerConnections map[net.Addr]*rpc.Client
	connectionMapMu sync.Mutex
	state           uint32
	done            bool
	doneChan        chan bool
	//fan-in channel for all rpc messages
	msgChan  chan *rpcMsg
	listener net.Listener
	// leader of the cluster
	leaderId net.Addr
	leaderMu sync.Mutex

	nodeState   nodeState
	leaderState leaderState
}

// Returns a new node in raft cluster
func NewNode(members []net.Addr, myAddr net.Addr) (*Node, error) {
	node := &Node{
		myAddr:          myAddr,
		members:         RemoveAddr(members, myAddr),
		peerConnections: make(map[net.Addr]*rpc.Client),
		state:           follower, // every node begins as a follower
		msgChan:         make(chan *rpcMsg),
		doneChan:        make(chan bool),
	}
	// start rpc server
	rpc.Register(node)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", myAddr.String())
	if e != nil {
		return node, e
	}
	node.listener = l
	go http.Serve(l, nil)
	go node.start()
	return node, nil
}

func (node *Node) getClient(addr net.Addr) *rpc.Client {
	node.connectionMapMu.Lock()
	defer node.connectionMapMu.Unlock()
	var client *rpc.Client
	if cl, ok := node.peerConnections[addr]; !ok {
		if client, err := rpc.DialHTTP("tcp", addr.String()); err != nil {
			node.peerConnections[addr] = client
		}
	} else {
		client = cl
	}
	return client
}

// execute as a follower
func (node *Node) follower() {
	heartBeatTimer := time.NewTimer(heartBeatTimeOut)
	for node.state == follower {
		select {
		case msg := <-node.msgChan:
			fmt.Println(msg)
		case <-heartBeatTimer.C:

		case <-node.doneChan:
		}
	}
}

//execute as a candidate
func (node *Node) candidate() {
	for node.state == candidate {
		select {
		case msg := <-node.msgChan:
			fmt.Println(msg)

		case <-node.doneChan:
		}
	}
}

//execute as a leader
func (node *Node) leader() {
	heartBeatTimer := time.NewTimer(heartBeatTimeOut)
	for node.state == leader {
		select {
		case msg := <-node.msgChan:
			fmt.Println(msg)

		case <-heartBeatTimer.C:

		case <-node.doneChan:
		}
	}

}

func (node *Node) start() {
	for node.done == false {
		switch node.state {
		case follower:
			node.follower()
		case candidate:
			node.candidate()
		case leader:
			node.leader()
		}
	}
}

func (node *Node) handleAppendEntriesReq(req *AppendEntriesReq) (*AppendEntriesResp, error) {
	var resp AppendEntriesResp
	return &resp, nil
}

func (node *Node) handleVoteReq(req *RequestVotesReq) (*RequestVotesResp, error) {
	var resp RequestVotesResp
	return &resp, nil
}

func (node *Node) processMsg(rpcMsg rpcMsg) {
	switch rpcMsg.msgType {
	case append_entries_req:
		resp, err := node.handleAppendEntriesReq(rpcMsg.msg.(*AppendEntriesReq))
		if err == nil {
			rpcMsg.respCh <- resp
		}
	case voting_req:
		resp, err := node.handleVoteReq(rpcMsg.msg.(*RequestVotesReq))
		if err == nil {
			rpcMsg.respCh <- resp
		}
	default:
	}
}

// RPC methods, fan-in all rpc requests in to rpc channel of node
func (node *Node) RpcAppendEntries(req *AppendEntriesReq, resp *AppendEntriesResp) error {
	msg := &rpcMsg{
		msgType: append_entries_req,
		msg:     req,
		respCh:  make(chan interface{}),
	}
	node.msgChan <- msg
	reply := <-msg.respCh
	resp = reply.(*AppendEntriesResp)
	return nil
}

func (node *Node) RpcVotingReq(req *RequestVotesReq, resp *RequestVotesResp) error {
	msg := &rpcMsg{
		msgType: append_entries_req,
		msg:     req,
		respCh:  make(chan interface{}),
	}
	node.msgChan <- msg
	reply := <-msg.respCh
	resp = reply.(*RequestVotesResp)
	return nil
}

func (node *Node) Kill() {
	node.done = true
	node.listener.Close()
	node.doneChan <- true
}
