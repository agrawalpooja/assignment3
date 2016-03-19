package raft
//import "github.com/cs733-iitb/cluster"
//import "github.com/cs733-iitb/cluster/mock"
import "testing"
import "fmt"
import "time"
func Test_conf(t *testing.T){
	nodes,_ := makeRafts()
	for _,v := range nodes{
		fmt.Printf("%T \n",v)
		fmt.Println("--->",v)
	}
	rn := nodes[0]
	rn.timeoutCh <- TimeoutEv{}
	//server2 := rn.server
	//server2.Outbox() <- &cluster.Envelope{Pid: 1, Msg: TimeoutEv{}}
	//rn = nodes[0]
	go func(){
		rn.processEvents()
	}()
	go func(){
		nodes[1].processEvents()
	}()
	go func(){
		nodes[2].processEvents()
	}()
	time.Sleep(time.Second*10)
	ldr := LeaderId(nodes)
	nodes[ldr-1].Append([]byte("foo"))
	time.Sleep(1*time.Second)
	for _, rnode:= range nodes {
		select { // to avoid blocking on channel.
		case ev := <- rnode.CommitChannel():
			fmt.Println(ev)
			ci:=ev.(CommitEv)
			if ci.err != nil {t.Fatal(ci.err)}
			fmt.Println(string(ci.data[ci.index].data))
			//fmt.Println("CommitCh-->  ",rn.sm)
			if string(ci.data[ci.index].data) != "foo" {
				t.Fatal("Got different data")
			}
			//continue
		//default: t.Fatal("Expected message on all nodes")
		}
	}
	//fmt.Println(rn.sm)
	//env := <-server2.Inbox()
	//fmt.Println("---->",env.Msg)
	fmt.Println("Done")
}	