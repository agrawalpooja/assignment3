package raft
import "fmt"
import "math"
type logEntries struct{
		Term int
		data []byte
	}
type VoteReqEv struct {
	CandidateId int
	Term int
	LastLogIndex int
	LastLogTerm int
	From int
	// etc
}
type VoteRespEv struct {
	Term int
	voteGranted bool
	From int
	// etc
}
type TimeoutEv struct {
}
type alarm struct{
	sec int
}
type AppendEv struct{
	data []byte
}
type CommitEv struct{
	index int
	data []logEntries
	err error
}
type Send struct{
	peerId int
	event interface{}
}
type AppendEntriesReqEv struct {
	Term int
	leaderId int
	prevLogIndex int
	prevLogTerm int
	entries []logEntries
	leaderCommit int
	From int
	// etc
}
type AppendEntriesRespEv struct {
	Term int
	success bool
	From int
	// etc
}
type LogStoreEv struct{
	index int
	entry logEntries
	From int
}

type StateMachine struct {
	id int // server id
	peers []int // other server ids
	Term int 
	votedFor int
	log []logEntries
	commitIndex int
	lastApplied int
	LastLogTerm int
	LastLogIndex int
	nextIndex []int
	matchIndex []int
	majority int
	state string
	numVotesGranted, numVotesDenied int
	votesRcvd []int // one per peer
	noAppendRes int
	ElectionTimeout int
	HeartbeatTimeout int
	// etc
}



func (sm *StateMachine) ProcessEvent (ev interface{}) []interface{}{
	var actions []interface{}
	switch ev.(type) {

		// handle heartbeat ?
	case AppendEntriesReqEv:
		cmd := ev.(AppendEntriesReqEv)
		if sm.state == "follower"{
			if cmd.Term < sm.Term || sm.log[cmd.prevLogIndex].Term != cmd.prevLogTerm{
				fmt.Println("**")
				actions = append(actions, Send{cmd.From, AppendEntriesRespEv{Term: sm.Term, success: false, From: sm.id}})
			}else{
				i := cmd.prevLogIndex
				for _,v := range cmd.entries{
					i = i + 1
					//sm.log[i] = v //logstore
					actions = append(actions, LogStoreEv{index: i, entry: v, From: sm.id})
				}
				if cmd.leaderCommit > sm.commitIndex{
					sm.commitIndex = int(math.Min(float64(cmd.leaderCommit), float64(i)))
				}
				if cmd.Term > sm.Term{
					sm.Term = cmd.Term
					sm.votedFor = 0
				}
				fmt.Println("***")
				actions = append(actions, Send{cmd.From, AppendEntriesRespEv{Term: sm.Term, success: true, From: sm.id}})
				actions = append(actions, alarm{sec: sm.ElectionTimeout})
			}
		}
		//check if Term changes ?
		if sm.state == "candidate"{
			sm.state = "follower"
			sm.Term = cmd.Term
			sm.votedFor = 0
			actions = append(actions, Send{cmd.From, AppendEntriesRespEv{Term: sm.Term, success: true, From: sm.id}})
			actions = append(actions, alarm{sec: sm.ElectionTimeout})
		}
		// do stuff with req
		//fmt.Printf("%v\n", cmd)

	case AppendEntriesRespEv:
		cmd := ev.(AppendEntriesRespEv)
		if sm.state == "leader"{
			if cmd.success == true{
				sm.noAppendRes++
				//update nextindex and matchindex // how? From id?
				sm.nextIndex[cmd.From] = sm.LastLogIndex + 1
				sm.matchIndex[cmd.From] = sm.LastLogIndex
				sm.majority = 2
				if sm.noAppendRes >= sm.majority{
					actions = append(actions, CommitEv{sm.LastLogIndex, sm.log[:sm.LastLogIndex+1], nil})
					sm.commitIndex = sm.LastLogIndex
				}
			}else{
				// two conditions
				if sm.Term < cmd.Term{
					sm.state = "follower"
					sm.Term = cmd.Term
					sm.votedFor = 0
				}else{
					sm.nextIndex[cmd.From]--
					prev := sm.nextIndex[cmd.From] - 1
					actions = append(actions, Send{cmd.From ,AppendEntriesReqEv{Term: sm.Term, leaderId: sm.id, prevLogIndex: prev, prevLogTerm: sm.log[prev].Term, entries: sm.log[prev:], leaderCommit: sm.commitIndex, From: sm.id}})
					actions = append(actions, alarm{sec: sm.ElectionTimeout})
				}
				//decrement nextIndex and retry ////////prevlogindex?
			}
		}
	
	// RequestVote RPC
	case VoteReqEv:
		cmd := ev.(VoteReqEv)
		var voteGranted bool
		if sm.state == "follower"{
			if cmd.Term < sm.Term{
				voteGranted = false
			}else if sm.votedFor == 0 || sm.votedFor == cmd.CandidateId{
					if cmd.LastLogTerm > sm.LastLogTerm{		//sm.log[len(sm.log)].Term
						sm.Term = cmd.Term
						sm.votedFor = cmd.CandidateId
						voteGranted = true
					}else if cmd.LastLogTerm == sm.LastLogTerm && cmd.LastLogIndex >= sm.LastLogIndex{		//len(sm.log)
						sm.Term = cmd.Term
						sm.votedFor = cmd.CandidateId
						voteGranted = true
					}else{
						voteGranted = false
					}
			}else{
				voteGranted =false
			}
			actions = append(actions, Send{cmd.From, VoteRespEv{Term: sm.Term, voteGranted: voteGranted, From: sm.id}})
			actions = append(actions, alarm{sec: sm.ElectionTimeout})
			// do stuff with req
			//fmt.Printf("%v\n", cmd)
		}else{										//leader or candidate
				if sm.Term < cmd.Term{
					sm.Term = cmd.Term
					sm.votedFor = 0
					sm.state = "follower" 
					actions = append(actions, alarm{sec: sm.ElectionTimeout})    //do we need resp here?
				}
		}
	case VoteRespEv:
		cmd := ev.(VoteRespEv)
		if sm.state == "candidate"{
			if sm.Term < cmd.Term{
				sm.Term = cmd.Term
				sm.votedFor = 0
				sm.state = "follower"
				actions = append(actions, alarm{sec: sm.ElectionTimeout})
			}else{
				if cmd.voteGranted == true{
					sm.numVotesGranted++
					sm.votesRcvd[cmd.From] = 1		// 1 for peer has voted yes
				}else if cmd.voteGranted == false{
					sm.numVotesDenied++		
					sm.votesRcvd[cmd.From] = -1		// -1 for peer has voted no
				}else{
					sm.votesRcvd[cmd.From] = 0		// 0 for peer has not voted
				}
				sm.majority = 2
				if sm.numVotesGranted >= sm.majority{
					sm.state = "leader"
					fmt.Println("leader elected")
					for _,i := range sm.peers{
						actions = append(actions, Send{i ,AppendEntriesReqEv{Term: sm.Term, leaderId: sm.id, leaderCommit: sm.commitIndex, From: sm.id}}) //to all
					}
					actions = append(actions, alarm{sec: sm.HeartbeatTimeout})  //heartbeat timer
				}
			}	
		}	

	case TimeoutEv:
		//cmd := ev.(TimeoutEv)
		//election timeout
		if sm.state == "follower" || sm.state == "candidate"{
			sm.state = "candidate"
			sm.Term++
			sm.votedFor = sm.id
			//reset election timer
			actions = append(actions, alarm{sec: sm.ElectionTimeout})
			//n := len(sm.log)
			for _,i := range sm.peers{
				actions = append(actions, Send{i, VoteReqEv{Term: sm.Term, CandidateId: sm.id, LastLogIndex: sm.LastLogIndex, LastLogTerm: sm.LastLogTerm, From: sm.id}})//to all			
			}
			
		}else{
			for _,i := range sm.peers{
				actions = append(actions, Send{i, AppendEntriesReqEv{Term: sm.Term, leaderId: sm.id, leaderCommit: sm.commitIndex, From: sm.id}})   //hearbeat timeout
				actions = append(actions, alarm{sec: sm.HeartbeatTimeout})
			}
			
		}

	case AppendEv:
		cmd := ev.(AppendEv)
		if sm.state == "follower"{
			actions = append(actions, AppendEv{data : cmd.data})  //forward to leader
		}else if sm.state == "leader"{
			actions = append(actions, LogStoreEv{index: sm.nextIndex[sm.id], entry: logEntries{sm.Term, cmd.data}, From: sm.id})
			for   _,i := range sm.peers{
				actions = append(actions, Send{i, AppendEntriesReqEv{Term: sm.Term, leaderId: sm.id, prevLogIndex: sm.nextIndex[sm.id]-1, prevLogTerm: sm.log[sm.nextIndex[sm.id]-1].Term, entries: []logEntries{{sm.Term, cmd.data}}, leaderCommit: sm.commitIndex, From: sm.id}})   //to all
				actions = append(actions, alarm{sec: sm.HeartbeatTimeout})
			}
			sm.nextIndex[sm.id]++
			sm.matchIndex[sm.id]++
		}

	case Send:


	case LogStoreEv:
		cmd := ev.(LogStoreEv)
		//fmt.Println(cmd)
		sm.log[cmd.index] = cmd.entry
		fmt.Println("log copied")
	// other cases
	default: println ("Unrecognized")
	}
	return actions
}