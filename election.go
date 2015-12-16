package election

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"time"
)

type ILeaderElection interface {
	GetSession(sessionName string)
	GetConsulClient()
	ElectLeader()
	IsLeader()
	CancelElection()
}

type LeaderElection struct {
	Session       string
	LeaderKey     string
	WatchWaitTime int
	StopElection  chan bool
}

func (le *LeaderElection) CancelElection() {
	le.StopElection <- true
}

func (le *LeaderElection) StepDown() error {
    if le.IsLeader() {
	client := le.GetConsulClient()
	agent, _ := client.Agent().Self()
	le.GetSession(le.LeaderKey)
        key := &api.KVPair{Key: le.LeaderKey, Value: []byte(agent["Config"]["NodeName"].(string)), Session: le.Session}
        released, _, err := client.KV().Release(key, nil)
        if !released || err != nil {
            return err
        } else {
            fmt.Println("Released leadership")
        }
    }
    return nil
}

func (le *LeaderElection) IsLeader() bool {
	client := le.GetConsulClient()
	agent, _ := client.Agent().Self()
	le.GetSession(le.LeaderKey)
	kv, _, err := client.KV().Get(le.LeaderKey, nil)
	if err != nil {
		fmt.Printf("Unable to check for leadership\n")
		return false
	}
	if kv == nil {
		fmt.Printf("Leadership key is missing")
		return false
	}
	return agent["Config"]["NodeName"] == string(kv.Value) && le.Session == kv.Session
}

func (le *LeaderElection) GetSession(sessionName string) {
	client := le.GetConsulClient()
	agent, _ := client.Agent().Self()
	sessions, _, err := client.Session().List(nil)
	for _, session := range sessions {
		if session.Name == sessionName && session.Node == agent["Config"]["NodeName"] {
			le.Session = session.ID
			break
		}
	}
	if le.Session == "" {
		fmt.Println("No sessions found, getting")
		sessionEntry := &api.SessionEntry{Name: sessionName}
		le.Session, _, err = client.Session().Create(sessionEntry, nil)
		if err != nil {
			panic(err)
		}
	}
}

func (le *LeaderElection) GetConsulClient() (client *api.Client) {
	config := api.DefaultConfig()
	client, _ = api.NewClient(config)
	return client
}

func (le *LeaderElection) ElectLeader() {
	client := le.GetConsulClient()
	agent, _ := client.Agent().Self()
	stop := false
	for !stop {
		select {
		case <-le.StopElection:
			stop = true
			fmt.Println("Stopping election")
		default:
			if !le.IsLeader() {

				le.GetSession(le.LeaderKey)

				pair := &api.KVPair{
					Key:     le.LeaderKey,
					Value:   []byte(agent["Config"]["NodeName"].(string)),
					Session: le.Session,
				}

				aquired, _, err := client.KV().Acquire(pair, nil)

				if aquired {
					fmt.Printf("%s is now the leader\n", agent["Config"]["NodeName"])
				}

				if err != nil {
					panic(err)
				}

			}

			kv, _, _ := client.KV().Get(le.LeaderKey, nil)

			if kv != nil && kv.Session != "" {
				fmt.Println("Current leader: ", string(kv.Value))
				fmt.Println("Leader Session: ", string(kv.Session))
			}

			time.Sleep(time.Duration(le.WatchWaitTime) * time.Second)
		}
	}
}