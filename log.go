package raft

type LogEntry struct{
	Term uint64

}

type Log []*LogEntry
