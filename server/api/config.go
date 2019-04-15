package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cloudkarafka/cloudkarafka-manager/config"
	"github.com/cloudkarafka/cloudkarafka-manager/metrics"
	"github.com/cloudkarafka/cloudkarafka-manager/processes"
	"goji.io/pat"
)

func badRequest(w http.ResponseWriter, err error, fn string) {
	fmt.Fprintf(os.Stderr, "[ERROR] api.%s: %s\n", fn, err)
	http.Error(w, "Cannot parse request body", http.StatusBadRequest)
}

func serverError(w http.ResponseWriter, err error, fn, msg string) {
	fmt.Fprintf(os.Stderr, "[ERROR] api.%s: %s\n", fn, err)
	http.Error(w, msg, http.StatusBadRequest)
}

func checkBrokerURP(brokerId int) {
	sleep := 5
	for i := 0; i < sleep*20; i++ {
		r, err := metrics.QueryBroker(brokerId, "kafka.server:type=ReplicaManager,name=UnderReplicatedPartitions", "Value", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] CheckURP: %s\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[INFO] URP status: broker=%d, URP=%d\n", brokerId, r[0].Value)
			if r[0].Value == 0 {
				return
			}
		}
		time.Sleep(time.Duration(sleep) * time.Second)
	}

}

// Update kafka config on all brokers, rolling
func UpdateKafkaConfigAll(w http.ResponseWriter, r *http.Request) {
	var changes map[string]string
	err := parseRequestBody(r, &changes)
	if err != nil {
		badRequest(w, err, "UpdateKafkaConfigAll")
		return
	}
	changesToApply := make(map[string]string)
	waitForURP := false
	for key, value := range changes {
		if config.IsDynamicClusterSetting(key) {
			changesToApply[key] = value
		} else if !config.IsDynamicBrokerSetting(key) {
			// If any change require kafka restart, wait for URP
			waitForURP = true
		}
	}
	if len(changesToApply) > 0 {
		if err := config.ReloadClusterConfig(changesToApply); err != nil {
			serverError(w, err, "UpdateKafkaConfigAll",
				fmt.Sprintf("Couldn't reload cluster-wide changed", err))
			return
		}
	}
	jsonData, _ := json.Marshal(changes)
	for brokerId, _ := range config.BrokerUrls {
		fmt.Fprintf(os.Stderr, "[INFO] Starting config update on broker %d\n", brokerId)
		url := fmt.Sprintf("%s/api/config/kafka/%d", config.BrokerUrls.MgmtUrl(brokerId), brokerId)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			serverError(w, err, "UpdateKafkaConfigAll", "Couldn't request config update on broker")
			return
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "Got status %d from broker %d: %s", resp.StatusCode, brokerId, body)
			fmt.Fprintf(w, "Got status %d from broker %d: %s", resp.StatusCode, brokerId, body)
		} else {
			fmt.Fprintf(os.Stderr, "[INFO] Completed config update on broker %d\n", brokerId)
			if waitForURP && len(config.BrokerUrls) > 1 {
				fmt.Fprintf(os.Stderr, "[INFO] Kafka was restarted on broker %d, waiting for URP\n", brokerId)
				checkBrokerURP(brokerId)
			}
		}
	}
}

// Update kafka config on one broker
func UpdateKafkaConfig(w http.ResponseWriter, r *http.Request) {
	brokerId, err := strconv.Atoi(pat.Param(r, "brokerId"))
	if err != nil {
		badRequest(w, err, "UpdateKafkaConfig")
		return
	}
	var changes map[string]string
	err = parseRequestBody(r, &changes)
	if err != nil {
		badRequest(w, err, "UpdateKafkaConfig")
		return
	}
	if len(changes) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	file, err := os.OpenFile(
		fmt.Sprintf("%s/config/server.properties", config.KafkaDir),
		os.O_RDWR, 0755)
	if err != nil {
		serverError(w, err, "UpdateKafkaConfig", "Couldn't find kafka config file")
		return
	}
	conf := config.ParseKafkaConfig(file)
	for k, v := range changes {
		conf.Update(k, v)
	}
	file.Seek(0, 0)
	if err := conf.Write(file); err != nil {
		serverError(w, err, "UpdateKafkaConfig", fmt.Sprintf("Couldn't write changes to config file", err))
		return
	}
	changesToApply := make(map[string]string)
	restartKafka := false
	for key, value := range changes {
		if config.IsDynamicBrokerSetting(key) {
			changesToApply[key] = value
		} else if !config.IsDynamicClusterSetting(key) {
			// If change is not cluster nor broker dynamic, kafka must be restarted
			restartKafka = true
		}
	}
	if len(changesToApply) > 0 {
		if err = config.ReloadBrokerConfig(brokerId, changesToApply); err != nil {
			serverError(w, err, "UpdateKafkaConfig",
				"Couldn't reload kafka config but changes are written to config file")
		}
	}
	if restartKafka {
		// We only stop kafka here since the systemd config will restart it automatically
		if err = processes.StopKafka(); err != nil {
			serverError(w, err, "UpdateKafkaConfig",
				"Couldn't restart kafka, only dynamic changes were applied")
		}
	}
}
