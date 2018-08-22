package api

import (
	"cloudkarafka-mgmt/dm"
	"cloudkarafka-mgmt/zookeeper"
	"github.com/gorilla/mux"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	replicationFactorRequired = errors.New("ERROR: must suply replication_factor and it must be numeric.")
	partitionsRequired        = errors.New("ERROR: must suply partitions and it must be numeric")
)

func Topics(w http.ResponseWriter, r *http.Request, p zookeeper.Permissions) {
	switch r.Method {
	case "GET":
		topics(w, p)
	case "POST":
		if !p.ClusterWrite() {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Insufficient privileges, requires cluster write.")
			return
		}
		t, err := decodeTopic(r)
		if err != nil {
			internalError(w, err.Error())
		} else {
			createTopic(w, t)
		}
	default:
		http.NotFound(w, r)
	}
}

func Topic(w http.ResponseWriter, r *http.Request, p zookeeper.Permissions) {
	vars := mux.Vars(r)
	switch r.Method {
	case "GET":
		if p.TopicRead(vars["topic"]) {
			getTopic(w, vars["topic"])
			break
		}
		fallthrough
	case "PUT":
		if p.ClusterWrite() {
			t, err := decodeTopic(r)
			if err != nil {
				internalError(w, err.Error())
			} else {
				updateTopic(w, vars["topic"], t)
			}
			break
		}
		fallthrough
	case "DELETE":
		if p.ClusterWrite() {
			deleteTopic(w, vars["topic"])
			break
		}
		fallthrough
	default:
		http.NotFound(w, r)
	}
}

func SpreadPartitions(w http.ResponseWriter, r *http.Request, p zookeeper.Permissions) {
	switch r.Method {
	case "POST":
		internalError(w, "Not yet implemented.")
		return
	/*	err := zookeeper.SpreadPartitionEvenly(vars["topic"])
		if err != nil {
			internalError(w, err.Error())
		} else {
			fmt.Fprintf(w, string("Partition reassignment in progress"))
		}*/
	default:
		http.NotFound(w, r)
	}
}

func TopicThroughput(w http.ResponseWriter, r *http.Request, p zookeeper.Permissions) {
	vars := mux.Vars(r)
	switch r.Method {
	case "GET":
		topic := vars["topic"]
		if p.TopicRead(topic) {
			in := dm.TopicBytesIn(topic)
			out := dm.TopicBytesOut(topic)
			WriteJson(w, map[string]dm.Series{"in": in, "out": out})
			break
		}
		fallthrough
	default:
		http.NotFound(w, r)
	}
}

func Config(w http.ResponseWriter, r *http.Request, p zookeeper.Permissions) {
	vars := mux.Vars(r)
	if !p.TopicRead(vars["topic"]) {
		http.NotFound(w, r)
		return
	}
	cfg, err := zookeeper.Config(vars["topic"])
	if err != nil {
		internalError(w, err.Error())
	} else {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, string(cfg))
	}
}

func decodeTopic(r *http.Request) (dm.T, error) {
	var (
		t   dm.T
		err error
	)
	switch r.Header.Get("content-type") {
	case "application/json":
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&t)
		defer r.Body.Close()
	default:
		err = r.ParseMultipartForm(512)
		t.Name = r.PostForm.Get("name")
		t.PartitionCount, err = strconv.Atoi(r.PostForm.Get("partition_count"))
		t.ReplicationFactor, err = strconv.Atoi(r.PostForm.Get("replication_factor"))
		if r.PostForm.Get("config") != "" {
			rows := strings.Split(r.PostForm.Get("config"), "\n")
			cfg := make(map[string]interface{})
			for _, r := range rows {
				row := strings.TrimSpace(r)
				if row == "" {
					continue
				}
				cols := strings.Split(row, "=")
				key := strings.Trim(cols[0], " \n\r")
				val := strings.Trim(cols[1], " \n\r")
				cfg[key] = val
			}
			t.Config = cfg
		}
	}
	return t, err
}

func getTopic(w http.ResponseWriter, name string) {
	t, err := dm.Topic(name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	WriteJson(w, t)
}

func deleteTopic(w http.ResponseWriter, topic string) {
	err := zookeeper.DeleteTopic(topic)
	if err != nil {
		internalError(w, err.Error())
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func topics(w http.ResponseWriter, p zookeeper.Permissions) {
	allTopics, err := zookeeper.Topics(p)
	var myTopics []string
	for _, t := range allTopics {
		if p.TopicRead(t) {
			myTopics = append(myTopics, t)
		}
	}
	if err != nil {
		internalError(w, err.Error())
	} else {
		WriteJson(w, myTopics)
	}
}

func createTopic(w http.ResponseWriter, t dm.T) {
	err := zookeeper.CreateTopic(t.Name, t.PartitionCount, t.ReplicationFactor, t.Config)
	if err != nil {
		internalError(w, err.Error())
		return
	}
	fmt.Println(t.Name)
	getTopic(w, t.Name)
}

func updateTopic(w http.ResponseWriter, name string, t dm.T) {
	err := zookeeper.UpdateTopic(name, t.PartitionCount, t.ReplicationFactor, t.Config)
	if err != nil {
		fmt.Println(err)
		internalError(w, err.Error())
		return
	}
	getTopic(w, name)
}
