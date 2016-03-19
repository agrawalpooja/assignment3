package raft
import "time"
import "github.com/cs733-iitb/cluster"
import "github.com/cs733-iitb/cluster/mock"
import "github.com/cs733-iitb/log"
import "reflect"
import "fmt"
import "encoding/gob"
import "math/rand"
//type Event interface{}
type RaftNode struct{
	id int
	ElectionTimeout int
	HeartbeatTimeout int
	LogDir string
	eventCh chan interface{}
	timeoutCh chan interface{}
	commitCh chan interface{}
	t *time.Timer
	server *mock.MockServer
	sm *StateMachine
}
type rConfig struct{
	ElectionTimeout int
	HeartbeatTimeout int
	Id int // this node's id. One of the cluster's entries should match.
	LogDir string // Log file directory for this node
}
func (rn *RaftNode) processEvents() {
	gob.Register(TimeoutEv{})
	gob.Register(VoteReqEv{})
	gob.Register(VoteRespEv{})
	gob.Register(AppendEntriesReqEv{})
	gob.Register(AppendEntriesRespEv{})
	gob.Register(alarm{})
	gob.Register(AppendEv{})
	gob.Register(LogStoreEv{})
	gob.Register(CommitEv{})
	for {
	//var ev interface{}
		select {
			//case ev = <- eventCh {ev = msg}
			//<- timeoutCh {ev = Timeout{}}			
		case <- rn.timeoutCh:
			tev := TimeoutEv{}
			actions := rn.sm.ProcessEvent(tev)
			rn.doActions(actions)	
		case aev := <- rn.eventCh:
			actions := rn.sm.ProcessEvent(aev)
			rn.doActions(actions)	
		case ev := <-rn.server.Inbox():
			actions := rn.sm.ProcessEvent(ev.Msg)
			rn.doActions(actions)		
		}
	}
}
func (rn *RaftNode) doActions(actions []interface{}){
	for i,v := range actions{
		Evtype := reflect.TypeOf(v).Name()
		switch(Evtype){
		case "Send":
			vv:=v.(Send)
			rn.server.Outbox() <- &cluster.Envelope{Pid: vv.peerId, MsgId: int64(i), Msg: vv.event}
			fmt.Println(vv.peerId,"  ",reflect.TypeOf(vv.event).Name())
			//fmt.Println(reflect.TypeOf(vv.event).Name())

		case "alarm":
			//reset timer
			fmt.Println(rn.id,"  ",Evtype)
			vv:=v.(alarm)
			//fmt.Print("timer reset  ")
			//fmt.Print(time.Now(),"  reset with val ",vv.sec," ")
			//fmt.Println(rn.id)
			rn.t.Stop()
			rn.t = time.AfterFunc(time.Duration(vv.sec) * time.Second, func(){
			//fmt.Print("expired after reset  ")
			//fmt.Print(time.Now(),"  reset with val ",vv.sec," ")
			//fmt.Println(rn.id)
			rn.t.Stop()
			rn.timeoutCh <- TimeoutEv{}
			})
			//rn.timeoutCh <- TimeoutEv{}

		case "AppendEv":
			fmt.Println(Evtype)
		case "LogStoreEv":
			fmt.Println(rn.id,"  ",Evtype)
			rn.eventCh <- v
			//lg,err := log.Open("mylog")
     		//defer lg.Close()

		case "CommitEv":
			fmt.Println(Evtype)
			vv:=v.(CommitEv)
			if vv.data[vv.index].data != nil{
				rn.commitCh <- v
			}
		case "StateStoreEv":
		}	
	}
}
func makeRafts() ([]*RaftNode, error){
	// init the communication layer.
	// Note, mock cluster doesn't need IP addresses. Only Ids
	var sm *StateMachine
	clconfig := cluster.Config{Peers:[]cluster.PeerConfig{
		{Id:1}, {Id:2}, {Id:3},
	}}
	cluster, err := mock.NewCluster(clconfig)
	if err != nil {return nil, err}

	// init the raft node layer
	nodes := make([]*RaftNode, len(clconfig.Peers))

	raftConfig := rConfig{
		ElectionTimeout: 5, // seconds
		HeartbeatTimeout: 2,
	}
	// Create a raft node, and give the corresponding "Server" object from the
	// cluster to help it communicate with the others.

	for id := 1; id <= 3; id++ {
		raftNode := New(id, raftConfig) // 
		// Give each raftNode its own "Server" from the cluster.
		raftNode.server = cluster.Servers[id]
		pid := cluster.Servers[id].Pid()
		peer := cluster.Servers[id].Peers()
		lg := make([]logEntries,100)
		ni := []int{1, 1, 1, 1, 1, 1}
		mi := []int{0, 0, 0, 0, 0, 0}
		vr := []int{0, 0, 0, 0, 0, 0}
		sm = &StateMachine{id: pid, peers: peer, state: "follower", 
							Term: 0,
							log: lg,
							votedFor: 0,
							commitIndex: 0,
							lastApplied: 1,
							LastLogTerm: 0,
							LastLogIndex: 1, 
							nextIndex: ni,
							matchIndex: mi,
							majority: 2,
							numVotesGranted: 0, numVotesDenied: 0,
							votesRcvd: vr, // one per peer
							noAppendRes: 0,
							ElectionTimeout: raftNode.ElectionTimeout, 
							HeartbeatTimeout: raftConfig.HeartbeatTimeout}	
		raftNode.sm = sm
		nodes[id-1] = raftNode
	}
	return nodes, nil
}
func New(myid int, config rConfig) (raftNode *RaftNode) {
	raftNode = new(RaftNode)
	raftNode.id = myid
	rand.Seed( time.Now().UTC().UnixNano() * int64(myid))
	raftNode.ElectionTimeout = rand.Intn(5)+config.ElectionTimeout
	raftNode.HeartbeatTimeout = config.HeartbeatTimeout
	raftNode.eventCh = make(chan interface{}, 10)
	raftNode.timeoutCh = make(chan interface{}, 10)
	raftNode.commitCh = make(chan interface{}, 10)
	raftNode.t = time.AfterFunc(time.Duration(raftNode.ElectionTimeout) * time.Second, func(){})
	return raftNode
}
func (rn *RaftNode) Append(adata []byte) {
	rn.eventCh <- AppendEv{data: adata}
}
func LeaderId(rafts []*RaftNode) (int){
	for _,rn := range rafts{
		if rn.sm.state == "leader"{
			return rn.id
		}
	}
	return -1
}
func (rn *RaftNode) Id() (int){
	return rn.id 
}
func (rn *RaftNode) CommitChannel() (chan interface{}){
	return rn.commitCh
}
func (rn *RaftNode) CommittedIndex() (int){
	return rn.sm.commitIndex
}
func mkLog() (*log.Log) {
	lg, _ := log.Open("./mylog")
	return lg
}
/*type Node interface {
// Client's message to Raft node
Append([]byte)
// A channel for client to listen on. What goes into Append must come out of here at some point.
CommitChannel() <- chan CommitInfo
// Last known committed index in the log. This could be -1 until the system stabilizes.
CommittedIndex() int
// Returns the data at a log index, or an error.
Get(index int) (err, []byte)
// Node's id
Id()
1
// Id of leader. -1 if unknown
LeaderId() int
// Signal to shut down all goroutines, stop sockets, flush log and close it, cancel timers.
Shutdown()
}*/