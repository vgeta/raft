package raft

import "net"

// rpc msg sent from leader to followers requesting
// to append enrties to log as well as used as hear beat
// to maintain it's authority over the cluster in which case
// log entries will be empty.
type AppendEntriesReq struct {
	Term            uint64
	LeaderId        net.Addr
	PrevLogIdx      uint64
	PrevLogTerm     uint64
	LogEntries      []*LogEntry
	LeaderCommitIdx uint64
}

// rpc msg sent from followers to leader, Current term as seen by
// the follower which can be used by leader to update itself if it lags
// behind, Success is set to true by follower if the prevlogIdx and prevLogterm
// matches the log entry in it's log and the leader's term is greater tham or
// equal to current term of follower, Which sums up to whether the follewer has
// accepted the appendEntries request.
type AppendEntriesResp struct {
	Term    uint64
	Success bool
}

// When a follower fails to receive appenfEntry msg from leader of current term
// with in configured election time out, it becames a candidate by incrementing
// it's current term, votes to itself and sends request for votes to it's peers.
//  On receiving majority of votes it becomes leader and sends appendEntry request
// to followers.
type RequestVotesReq struct {
	Term         uint64
	CandidateId  net.Addr
	LastLogIndex uint64
	LastLogTerm  uint64
}

// rpc msgs sent from follower to candidate, Follower grants vote if candidate's term is
// grater than equal to it's current term and the lastlog term is greater than
// it's log entry, in case they are equal it grants vote if the candidate's log is longer
// than it's own. Term is set to the current term of follower. This can be used by the
// candidate to update itself if it lags behind.
type RequestVotesResp struct {
	Term        uint64
	VoteGranted bool
}

const (
	APPEND_ENTRIES_REQ = iota
	VOTING_REQ
)
