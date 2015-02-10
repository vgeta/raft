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
	leaderId      net.Addr
	leaderMu      sync.Mutex
	leaderContact time.Time

	//state maintained by all nodes in raft cluster
	// persistent values
	currentTerm uint64
	votedFor    net.Addr
	log         Log
	// volatile values
	commitIdx   uint64
	lastApplied uint64

	//state maintained by leader of the cluster
	nextIndex  map[net.Addr]uint64
	matchIndex map[net.Addr]uint64
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
			node.processMsg(msg)
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
			node.processMsg(msg)
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
			node.processMsg(msg)
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

// On receiving appenEntries req
// 1. Check if the request term is less than the current term, reject it if so
// 2. if req term is greater , Accept the leader and become follwer
func (node *Node) handleAppendEntriesReq(req *AppendEntriesReq) (*AppendEntriesResp, error) {
	resp := &AppendEntriesResp{
		Term: node.currentTerm,
	}
	if node.currentTerm > req.Term {
		return resp, nil
	}
	//TODO : have to do safe read/update, also persist them !!
	if node.currentTerm < req.Term {
		node.currentTerm = req.Term
		node.state = follower
		resp.Term = req.Term
		node.votedFor = nil
	}
	// keep the leader id up-to-date
	node.leaderId = req.LeaderId
	node.leaderContact = time.Now()
	return resp, nil
}

// On receiving request to vote
// 1. check if request term is less than current term, reject it if so
// 2. else update node's current term
func (node *Node) handleVoteReq(req *RequestVotesReq) (*RequestVotesResp, error) {
	resp := &RequestVotesResp{
		Term: node.currentTerm,
	}
	if node.currentTerm > req.Term {
		return resp, nil
	}
	if node.currentTerm < req.Term {
		node.currentTerm = req.Term
		node.state = follower
		resp.Term = req.Term
		node.votedFor = nil
	}
	if len(node.votedFor.String()) == 0 || node.votedFor.String() == req.CandidateId.String() {
		node.votedFor = req.CandidateId
		resp.VoteGranted = true
	}
	//TODO: don't give your vote so easily :)
	return resp, nil
}

func (node *Node) sendAppendReqs() {
	appendReq := &AppendEntriesReq{
		Term:     node.currentTerm,
		LeaderId: node.myAddr,
	}
	for _, peer := range node.members {
		go func() {
			var resp AppendEntriesResp
			cl := node.getClient(peer)
			cl.Call("Node.RpcAppendEntries", appendReq, &resp)
		}()
	}
}

func (node *Node) sendVoteReqs() {
	voteReq := &RequestVotesReq{
		Term:        node.currentTerm,
		CandidateId: node.myAddr,
	}
	for _, peer := range node.members {
		go func() {
			var resp RequestVotesResp
			cl := node.getClient(peer)
			cl.Call("Node.RpcVotingReq", voteReq, &resp)
		}()
	}
}

func (node *Node) processMsg(rpcMsg *rpcMsg) {
	switch rpcMsg.msgType {
	case append_entries_req:
		resp, err := node.handleAppendEntriesReq(rpcMsg.msg.(*AppendEntriesReq))
		if err != nil {
			fmt.Println("processMsg err", err)
		}
		rpcMsg.respCh <- resp
	case voting_req:
		resp, err := node.handleVoteReq(rpcMsg.msg.(*RequestVotesReq))
		if err != nil {
			fmt.Println("processMsg err", err)
		}
		rpcMsg.respCh <- resp
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
		msgType: voting_req,
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
