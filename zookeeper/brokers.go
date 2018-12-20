package zookeeper

import (
	"fmt"
	"strconv"
)

type B struct {
	Version   int      `json:"-"`
	JmxPort   int      `json:"jmx_port"`
	Timestamp string   `json:"timestamp"`
	Endpoints []string `json:"endpoints"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Id        int      `json:"id"`
}

func Brokers() ([]int, error) {
	stringIds, err := all("/brokers/ids", func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(stringIds))
	for i, id := range stringIds {
		if intId, err := strconv.Atoi(id); err == nil {
			ids[i] = intId
		}
	}
	return ids, nil
}

func Broker(id int) (B, error) {
	var b B
	err := get(fmt.Sprintf("/brokers/ids/%d", id), &b)
	b.Id = id
	return b, err
}
